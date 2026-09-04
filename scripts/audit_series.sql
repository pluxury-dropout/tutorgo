-- Ревизия старых серий уроков перед переносом series_id → rule_id (п. 7.6 спеки
-- docs/specs/2026-08-31-calendar-events-and-scheduling-flow.md).
--
-- Только читает. Показывает по каждой серии её фактическую форму — дни недели,
-- время, интервал — и сколько уроков отрезала бы граница «не дальше N месяцев».
-- Смысл в том, чтобы решение о чистке принималось по данным: правило,
-- восстановленное по серии с хвостом до 2030 года, окажется бессрочным, и
-- первое же «изменить все следующие» размножит уроки.
--
-- Запуск:  psql "$DB_URL" -X -f scripts/audit_series.sql
--
-- Зона жёстко Asia/Almaty: дни недели и время нужны как стенные часы
-- репетитора, а в UTC ночные слоты уезжают на соседний день.

\pset border 2
\timing off

WITH gaps AS (
    SELECT series_id,
           (scheduled_at AT TIME ZONE 'Asia/Almaty')::date
             - lag((scheduled_at AT TIME ZONE 'Asia/Almaty')::date)
               OVER (PARTITION BY series_id ORDER BY scheduled_at) AS gap_days
    FROM lessons
    WHERE series_id IS NOT NULL
),
median AS (
    SELECT series_id,
           percentile_disc(0.5) WITHIN GROUP (ORDER BY gap_days) AS median_gap
    FROM gaps
    WHERE gap_days IS NOT NULL AND gap_days > 0
    GROUP BY 1
),
agg AS (
    SELECT l.series_id,
           c.tutor_id,
           c.subject,
           COALESCE(st.first_name, '(группа)')                       AS student,
           count(*)                                                  AS lessons,
           count(*) FILTER (WHERE l.scheduled_at > now())            AS future,
           count(*) FILTER (WHERE l.status = 'cancelled')            AS cancelled,
           -- Кандидаты на обрезку при двух вариантах границы.
           count(*) FILTER (WHERE l.scheduled_at > now() + interval '12 months') AS beyond_12m,
           count(*) FILTER (WHERE l.scheduled_at > now() + interval '6 months')  AS beyond_6m,
           -- Следы жизни у будущих уроков: посещаемость, заметки, начатая
           -- комната. Такие резать вслепую нельзя.
           count(*) FILTER (
               WHERE l.scheduled_at > now() + interval '12 months'
                 AND (l.notes <> '' OR l.room_started_at IS NOT NULL
                      OR EXISTS (SELECT 1 FROM lesson_attendances a WHERE a.lesson_id = l.id))
           )                                                         AS touched_beyond_12m,
           array_agg(DISTINCT extract(isodow FROM l.scheduled_at AT TIME ZONE 'Asia/Almaty')::int
                     ORDER BY extract(isodow FROM l.scheduled_at AT TIME ZONE 'Asia/Almaty')::int) AS weekdays,
           count(DISTINCT to_char(l.scheduled_at AT TIME ZONE 'Asia/Almaty', 'HH24:MI')) AS distinct_times,
           min(to_char(l.scheduled_at AT TIME ZONE 'Asia/Almaty', 'HH24:MI'))            AS time_local,
           min(l.scheduled_at)::date                                 AS first_day,
           max(l.scheduled_at)::date                                 AS last_day
    FROM lessons l
    JOIN courses c  ON c.id = l.course_id
    LEFT JOIN students st ON st.id = c.student_id
    WHERE l.series_id IS NOT NULL
    GROUP BY 1, 2, 3, 4
)
SELECT left(a.series_id::text, 8)                     AS series,
       left(a.tutor_id::text, 8)                      AS tutor,
       left(a.subject || ' / ' || a.student, 24)      AS course,
       a.lessons,
       a.future,
       a.cancelled                                    AS cancl,
       a.weekdays                                     AS dows,
       CASE WHEN a.distinct_times > 1
            THEN a.time_local || ' (+' || (a.distinct_times - 1) || ')'
            ELSE a.time_local END                     AS time_local,
       m.median_gap                                   AS gap,
       a.first_day,
       a.last_day,
       a.beyond_6m                                    AS cut_6m,
       a.beyond_12m                                   AS cut_12m,
       a.touched_beyond_12m                           AS touched
FROM agg a
LEFT JOIN median m ON m.series_id = a.series_id
ORDER BY a.lessons DESC, a.last_day DESC;

-- Сводка: сколько всего отрежется при каждой границе.
SELECT count(DISTINCT series_id)                                              AS series_total,
       count(*)                                                              AS lessons_total,
       count(*) FILTER (WHERE scheduled_at > now())                          AS future_total,
       count(*) FILTER (WHERE scheduled_at > now() + interval '6 months')    AS cut_at_6m,
       count(*) FILTER (WHERE scheduled_at > now() + interval '12 months')   AS cut_at_12m,
       count(*) FILTER (
           WHERE scheduled_at > now() + interval '12 months'
             AND (notes <> '' OR room_started_at IS NOT NULL
                  OR EXISTS (SELECT 1 FROM lesson_attendances a WHERE a.lesson_id = lessons.id))
       )                                                                     AS touched_beyond_12m
FROM lessons
WHERE series_id IS NOT NULL;

-- +goose Up
-- Одноразовый перенос старых серий (lessons.series_id) на правила повторений
-- (п. 7.6 спеки docs/specs/2026-08-31-calendar-events-and-scheduling-flow.md).
--
-- Ревизия (scripts/audit_series.sql) показала, чем эти серии стали за год: у
-- половины время плавает (до 17 разных значений в одной серии), дни недели
-- расползлись на всю неделю, медианный интервал 2–3 дня. Восстановленное
-- правило описывает не серию, а её самую частую форму — поэтому всё, что в
-- эту форму не попало, помечается is_override и материализацией не трогается.
--
-- Три защиты от размножения уроков:
--   1. ends_on = дата последнего урока после обрезки: правило не бессрочное;
--   2. materialized_until = ends_on: Materialize считает вперёд от этой границы
--      (см. service/recurrence.go), поэтому пропуски внутри серии не заполняются;
--   3. is_override на отклонениях: «изменить все следующие» их не снесёт.

-- Шаг 0. Полный снимок — из него Down восстанавливает и удалённые уроки, и
-- привязку к сериям. Таблицу можно дропнуть руками, когда перенос устоится.
CREATE TABLE lessons_series_backup AS
SELECT * FROM lessons WHERE series_id IS NOT NULL;

-- Шаг 1. Обрезка хвостов. Это артефакт бага, который чинила фаза 0: клиент
-- раскатывал ровно 200 уроков, поэтому серии тянутся до 2028–2030 годов.
-- Режется только будущее дальше полугода; ревизия подтвердила, что ни у одного
-- из этих уроков нет ни заметок, ни посещаемости, ни начатой комнаты.
DELETE FROM lessons
WHERE series_id IS NOT NULL
  AND scheduled_at > now() + interval '6 months';

-- Шаг 2. Правило на серию. id правила = series_id: маппинг не нужен, а
-- вероятность коллизии двух UUID пренебрежима.
INSERT INTO recurrence_rules
    (id, tutor_id, freq, interval_n, byweekday, time_local, tz,
     duration_minutes, starts_on, ends_on, materialized_until)
SELECT s.series_id,
       s.tutor_id,
       'weekly',
       1,
       COALESCE(d.byweekday, '{}')::smallint[],
       s.time_local,
       'Asia/Almaty',
       s.duration_minutes,
       s.starts_on,
       s.ends_on,
       s.ends_on
FROM (
    SELECT l.series_id,
           c.tutor_id,
           mode() WITHIN GROUP (ORDER BY date_trunc('minute', (l.scheduled_at AT TIME ZONE 'Asia/Almaty'))::time) AS time_local,
           mode() WITHIN GROUP (ORDER BY l.duration_minutes)                    AS duration_minutes,
           min((l.scheduled_at AT TIME ZONE 'Asia/Almaty')::date)               AS starts_on,
           max((l.scheduled_at AT TIME ZONE 'Asia/Almaty')::date)               AS ends_on
    FROM lessons l
    JOIN courses c ON c.id = l.course_id
    WHERE l.series_id IS NOT NULL
    GROUP BY 1, 2
) s
LEFT JOIN (
    -- День недели попадает в правило, если на него приходится хотя бы десятая
    -- часть занятий серии: разовые переносы в правило не превращаются.
    SELECT series_id, array_agg(dow ORDER BY dow) AS byweekday
    FROM (
        SELECT series_id,
               extract(isodow FROM scheduled_at AT TIME ZONE 'Asia/Almaty')::int AS dow,
               count(*)::numeric / sum(count(*)) OVER (PARTITION BY series_id)   AS share
        FROM lessons
        WHERE series_id IS NOT NULL
        GROUP BY 1, 2
    ) t
    WHERE share >= 0.10
    GROUP BY 1
) d ON d.series_id = s.series_id;

-- Шаг 3. Привязка уроков. Уникальный индекс (rule_id, occurrence_date) держит
-- одно вхождение на дату, а в старых сериях есть дни с двумя занятиями —
-- второе такое остаётся одиночным уроком, без правила.
WITH ranked AS (
    SELECT id,
           series_id,
           (scheduled_at AT TIME ZONE 'Asia/Almaty')::date AS occ,
           row_number() OVER (
               PARTITION BY series_id, (scheduled_at AT TIME ZONE 'Asia/Almaty')::date
               ORDER BY scheduled_at
           ) AS rn
    FROM lessons
    WHERE series_id IS NOT NULL
)
UPDATE lessons l
SET rule_id = r.series_id, occurrence_date = r.occ
FROM ranked r
WHERE r.id = l.id AND r.rn = 1;

-- Шаг 4. Всё, что выпадает из восстановленной формы, — вручную правленное.
-- Отменённые тоже: тумбстоун обязан пережить «изменить все следующие».
UPDATE lessons l
SET is_override = TRUE
FROM recurrence_rules r
WHERE l.rule_id = r.id
  AND (
        NOT (extract(isodow FROM l.scheduled_at AT TIME ZONE 'Asia/Almaty')::smallint = ANY (r.byweekday))
        OR date_trunc('minute', (l.scheduled_at AT TIME ZONE 'Asia/Almaty'))::time <> r.time_local
        OR l.duration_minutes <> r.duration_minutes
        OR l.status = 'cancelled'
      );

-- Шаг 5. Снять со старого механизма: у урока должно быть заполнено ровно одно
-- из полей, иначе фронт покажет старый SeriesDialog поверх новых правил.
UPDATE lessons SET series_id = NULL WHERE series_id IS NOT NULL;

-- +goose Down
-- Возвращаем всё из снимка: и привязку к сериям, и удалённые хвосты.
UPDATE lessons l
SET series_id = b.series_id, rule_id = NULL, occurrence_date = NULL, is_override = b.is_override
FROM lessons_series_backup b
WHERE b.id = l.id;

INSERT INTO lessons SELECT * FROM lessons_series_backup b
WHERE NOT EXISTS (SELECT 1 FROM lessons l WHERE l.id = b.id);

DELETE FROM recurrence_rules r
WHERE EXISTS (SELECT 1 FROM lessons_series_backup b WHERE b.series_id = r.id);

DROP TABLE lessons_series_backup;

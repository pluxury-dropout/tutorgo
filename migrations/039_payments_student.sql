-- +goose Up
-- Платёж адресный: долг и баланс — свойства человека, а не курса. Групповой
-- курс (student_id IS NULL) вообще не даёт понять, кто из пяти заплатил.
ALTER TABLE payments ADD COLUMN student_id UUID REFERENCES students(id) ON DELETE CASCADE;

-- Индивидуальные курсы бэкфиллятся однозначно: у курса ровно один ученик.
UPDATE payments p
   SET student_id = c.student_id
  FROM courses c
 WHERE c.id = p.course_id AND c.student_id IS NOT NULL;

-- Групповые платежи адресата не имеют и восстановить его неоткуда: остаются
-- NULL и в баланс конкретного ученика не входят (спека, п. 3.4).

CREATE INDEX idx_payments_student ON payments(student_id) WHERE student_id IS NOT NULL;

-- Периоды участия (спека, п. 3.9): с какого числа уроки ученика сгорают; по
-- какое — left_at, заведённый миграцией 038 (фаза 1.5). Без enrolled_at ученик,
-- добавленный в ноябре, задолжает за все занятия с сентября: сгорает каждый
-- состоявшийся урок курса, а строка не помнит, когда участник появился.
ALTER TABLE course_enrollments ADD COLUMN enrolled_at TIMESTAMPTZ;

-- Строки course_enrollments бывают только у групповых курсов, а групповым
-- платежам student_id здесь не выдаётся (см. выше) — значит искать «первый
-- платёж этого ученика» бессмысленно, подзапрос вернул бы NULL для всех строк.
-- Единственный след участия, который есть у групп сейчас, — посещаемость;
-- started_at остаётся для тех, кого ни разу не отмечали (спека, п. 3.7).
UPDATE course_enrollments ce
   SET enrolled_at = COALESCE(
         (SELECT min(l.scheduled_at)
            FROM lesson_attendances la
            JOIN lessons l ON l.id = la.lesson_id
           WHERE l.course_id = ce.course_id AND la.student_id = ce.student_id),
         c.started_at)
  FROM courses c
 WHERE c.id = ce.course_id;

ALTER TABLE course_enrollments
  ALTER COLUMN enrolled_at SET NOT NULL,
  ALTER COLUMN enrolled_at SET DEFAULT NOW();

-- Заморозка (спека, п. 3.9 и 6.9): интервал висит на ученике, без course_id —
-- «уехал на месяц» закрывает все его предметы и группы одним действием.
CREATE TABLE student_pauses (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID        NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    starts_on  DATE        NOT NULL,
    ends_on    DATE        NOT NULL,
    reason     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (ends_on >= starts_on)
);

CREATE INDEX idx_pauses_student ON student_pauses(student_id, starts_on);

-- +goose Down
DROP TABLE IF EXISTS student_pauses;
ALTER TABLE course_enrollments DROP COLUMN enrolled_at;
DROP INDEX IF EXISTS idx_payments_student;
ALTER TABLE payments DROP COLUMN student_id;

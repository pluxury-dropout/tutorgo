-- +goose Up
-- Неявное создание курса по паре «ученик + предмет» (get-or-create) под
-- READ COMMITTED от гонки не защищено: два параллельных запроса (двойной клик,
-- две вкладки) оба увидят пустой SELECT до чужого INSERT. Уникальность на
-- тройку — то, что превращает гонку в ON CONFLICT DO NOTHING.
--
-- Индекс частичный: у группового курса student_id IS NULL и одинаковых групп по
-- одному предмету может быть сколько угодно, а архивный курс (is_active = FALSE)
-- не должен мешать завести новый с тем же предметом.
CREATE UNIQUE INDEX idx_courses_tutor_student_subject ON courses(tutor_id, student_id, subject)
    WHERE is_active AND student_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_courses_tutor_student_subject;

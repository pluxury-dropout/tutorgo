-- +goose Up
-- Заменяем per-lesson LessonTask на markdown-ДЗ уровня курса.
DROP TABLE IF EXISTS lesson_tasks;
ALTER TABLE courses ADD COLUMN homework TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE courses DROP COLUMN homework;
-- lesson_tasks намеренно не восстанавливаем — фича удалена.

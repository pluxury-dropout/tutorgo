-- +goose Up
-- Токен ICS-подписки: по нему публичная ручка отдаёт расписание репетитора без
-- авторизации, поэтому он и есть секрет. Пустая строка — подписки нет; выдаётся
-- по запросу и отзывается обнулением, после чего старая ссылка перестаёт
-- работать.
ALTER TABLE tutors ADD COLUMN ics_token TEXT NOT NULL DEFAULT '';

-- Частичный: пустых значений будет столько же, сколько репетиторов без подписки.
CREATE UNIQUE INDEX idx_tutors_ics_token ON tutors(ics_token) WHERE ics_token <> '';

-- +goose Down
DROP INDEX IF EXISTS idx_tutors_ics_token;
ALTER TABLE tutors DROP COLUMN ics_token;

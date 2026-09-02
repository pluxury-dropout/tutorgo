-- +goose Up
-- Отменённое вхождение серии событий должно остаться строкой: удали её целиком
-- — и ночная материализация создаст «спортзал» заново, потому что дата снова
-- свободна. У урока эту роль играет status = 'cancelled', у события статуса
-- нет, поэтому отдельный флаг.
ALTER TABLE events ADD COLUMN cancelled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE events DROP COLUMN cancelled;

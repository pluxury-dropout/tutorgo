-- +goose Up
ALTER TABLE lessons ADD COLUMN room_started_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE lessons DROP COLUMN room_started_at;
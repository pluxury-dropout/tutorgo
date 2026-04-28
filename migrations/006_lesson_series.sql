-- +goose Up
ALTER TABLE lessons ADD COLUMN series_id UUID NULL;
CREATE INDEX idx_lessons_series_id ON lessons(series_id) WHERE series_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_lessons_series_id;
ALTER TABLE lessons DROP COLUMN series_id;

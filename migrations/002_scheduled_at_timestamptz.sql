-- +goose Up
ALTER TABLE lessons
    ALTER COLUMN scheduled_at TYPE TIMESTAMPTZ
    USING scheduled_at AT TIME ZONE 'UTC';

-- +goose Down
ALTER TABLE lessons
    ALTER COLUMN scheduled_at TYPE TIMESTAMP
    USING scheduled_at AT TIME ZONE 'UTC';

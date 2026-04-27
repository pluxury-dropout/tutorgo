-- +goose Up

ALTER TABLE payments
    ALTER COLUMN paid_at TYPE TIMESTAMPTZ
    USING paid_at AT TIME ZONE 'UTC';

ALTER TABLE courses
    ALTER COLUMN ended_at TYPE TIMESTAMPTZ
    USING ended_at AT TIME ZONE 'UTC',
    ALTER COLUMN started_at TYPE TIMESTAMPTZ
    USING started_at AT TIME ZONE 'UTC';
    

-- +goose Down
ALTER TABLE payments
    ALTER COLUMN paid_at TYPE TIMESTAMP USING paid_at AT TIME ZONE 'UTC';

ALTER TABLE courses
    ALTER COLUMN ended_at   TYPE TIMESTAMP USING ended_at   AT TIME ZONE 'UTC',
    ALTER COLUMN started_at TYPE TIMESTAMP USING started_at AT TIME ZONE 'UTC';

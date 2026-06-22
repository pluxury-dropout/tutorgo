-- +goose Up
-- Kanban tasks have no calendar slot; scheduled_at/duration become optional.
ALTER TABLE tasks ALTER COLUMN scheduled_at DROP NOT NULL;
ALTER TABLE tasks ALTER COLUMN duration_minutes DROP NOT NULL;

-- +goose Down
ALTER TABLE tasks ALTER COLUMN scheduled_at SET NOT NULL;
ALTER TABLE tasks ALTER COLUMN duration_minutes SET NOT NULL;

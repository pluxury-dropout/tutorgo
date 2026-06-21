-- +goose Up
ALTER TABLE tasks ADD COLUMN status TEXT NOT NULL DEFAULT 'not_urgent';
UPDATE tasks SET status = 'done' WHERE done = true;
ALTER TABLE tasks DROP COLUMN done;

-- +goose Down
ALTER TABLE tasks ADD COLUMN done BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE tasks SET done = true WHERE status = 'done';
ALTER TABLE tasks DROP COLUMN status;

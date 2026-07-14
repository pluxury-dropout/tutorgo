-- +goose Up
ALTER TABLE boards ALTER COLUMN course_id DROP NOT NULL;
CREATE UNIQUE INDEX idx_boards_trial ON boards (tutor_id) WHERE course_id IS NULL;

-- +goose Down
DROP INDEX idx_boards_trial;
DELETE FROM boards WHERE course_id IS NULL;
ALTER TABLE boards ALTER COLUMN course_id SET NOT NULL;

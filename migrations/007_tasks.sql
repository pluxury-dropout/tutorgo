-- +goose Up
CREATE TABLE tasks (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id         UUID        NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    title            TEXT        NOT NULL,
    scheduled_at     TIMESTAMPTZ NOT NULL,
    duration_minutes INT         NOT NULL DEFAULT 30,
    done             BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_tasks_tutor_id ON tasks(tutor_id);

-- +goose Down
DROP TABLE tasks;

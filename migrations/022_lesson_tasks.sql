-- +goose Up
CREATE TABLE lesson_tasks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id   UUID NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    done        BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX lesson_tasks_lesson_id_idx ON lesson_tasks(lesson_id);

-- +goose Down
DROP TABLE IF EXISTS lesson_tasks;

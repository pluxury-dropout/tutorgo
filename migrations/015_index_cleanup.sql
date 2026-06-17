-- +goose Up
-- idx_tasks_tutor_id is superseded by idx_tasks_tutor_scheduled(tutor_id, scheduled_at).
DROP INDEX IF EXISTS idx_tasks_tutor_id;

-- GetMonthlyExpected scans lessons by scheduled_at range across all courses.
-- idx_lessons_course_active(course_id, scheduled_at) has course_id as the leading column,
-- forcing a full index scan when course_id is not filtered. This index puts scheduled_at first.
CREATE INDEX IF NOT EXISTS idx_lessons_scheduled_active
    ON lessons(scheduled_at, course_id)
    WHERE status <> 'cancelled';

-- +goose Down
CREATE INDEX IF NOT EXISTS idx_tasks_tutor_id ON tasks(tutor_id);
DROP INDEX IF EXISTS idx_lessons_scheduled_active;

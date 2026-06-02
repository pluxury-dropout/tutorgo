-- +goose Up
-- Speeds up the ranked CTE in GetCalendar: scans all non-cancelled lessons
-- per course to compute ROW_NUMBER(). The existing idx_lessons_course_scheduled
-- covers (course_id, scheduled_at) but does not filter on status, so Postgres
-- reads and discards cancelled rows. This partial index eliminates that waste.
CREATE INDEX IF NOT EXISTS idx_lessons_course_active
    ON lessons(course_id, scheduled_at)
    WHERE status <> 'cancelled';

-- GetByRange filters tasks by tutor_id + scheduled_at range. The existing
-- idx_tasks_tutor_id is single-column; this composite covers the range too.
CREATE INDEX IF NOT EXISTS idx_tasks_tutor_scheduled
    ON tasks(tutor_id, scheduled_at);

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_tutor_scheduled;
DROP INDEX IF EXISTS idx_lessons_course_active;

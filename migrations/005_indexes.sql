-- +goose Up
-- Performance indexes derived from real query paths in repository/*.go.
-- Postgres does NOT auto-create indexes on FK columns; everything below
-- closes a gap that currently forces a Seq Scan or a sort.

-- Tenant scope: every API call filters by tutor_id (courses.GetAll, students.GetAll, JOINs).
CREATE INDEX IF NOT EXISTS idx_courses_tutor  ON courses(tutor_id);
CREATE INDEX IF NOT EXISTS idx_students_tutor ON students(tutor_id);

-- Calendar (lesson.go:GetCalendar) and per-course list (lesson.go:GetByCourse):
-- composite serves WHERE course_id = $1 ORDER BY scheduled_at as a single index scan.
CREATE INDEX IF NOT EXISTS idx_lessons_course_scheduled ON lessons(course_id, scheduled_at);

-- Payments (payment.go:GetByCourse, GetAllByTutor): JOIN by course_id then sort by paid_at DESC.
CREATE INDEX IF NOT EXISTS idx_payments_course_paid_at ON payments(course_id, paid_at DESC);

-- Reverse lookup for course.go:GetByStudent — UNIQUE(course_id, student_id) does NOT cover this direction.
CREATE INDEX IF NOT EXISTS idx_enrollments_student ON course_enrollments(student_id);

-- Partial index for lesson.go:AutoComplete. Stores only rows the background job can touch
-- (status='scheduled'). scheduled_at is the leading column so the range comparison
-- "scheduled_at + duration_minutes*interval < NOW()" becomes an index scan instead of a Seq Scan.
-- The predicate is IMMUTABLE (a literal equality), as Postgres requires.
CREATE INDEX IF NOT EXISTS idx_lessons_pending
    ON lessons(scheduled_at)
    WHERE status = 'scheduled';

-- +goose Down
DROP INDEX IF EXISTS idx_lessons_pending;
DROP INDEX IF EXISTS idx_enrollments_student;
DROP INDEX IF EXISTS idx_payments_course_paid_at;
DROP INDEX IF EXISTS idx_lessons_course_scheduled;
DROP INDEX IF EXISTS idx_students_tutor;
DROP INDEX IF EXISTS idx_courses_tutor;

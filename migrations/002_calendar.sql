-- +goose Up
-- Calendar feature: groups, enrollments, attendance

ALTER TABLE courses ALTER COLUMN student_id DROP NOT NULL;

CREATE TABLE course_enrollments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id  UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    UNIQUE(course_id, student_id)
);

CREATE TABLE lesson_attendances (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id  UUID NOT NULL REFERENCES lessons(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    status     VARCHAR(10) NOT NULL CHECK (status IN ('present', 'absent')),
    UNIQUE(lesson_id, student_id)
);

-- +goose Down
DROP TABLE IF EXISTS lesson_attendances;
DROP TABLE IF EXISTS course_enrollments;
ALTER TABLE courses ALTER COLUMN student_id SET NOT NULL;

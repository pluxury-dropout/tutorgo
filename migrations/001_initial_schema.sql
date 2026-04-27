-- +goose Up

CREATE TABLE tutors (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    first_name    TEXT NOT NULL,
    last_name     TEXT NOT NULL,
    phone         TEXT
);

CREATE TABLE students (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id   UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_name  TEXT,
    phone      TEXT,
    email      TEXT,
    notes      TEXT,
    active     BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE courses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id      UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    tutor_id        UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    subject         TEXT NOT NULL,
    price_per_lesson NUMERIC(10,2) NOT NULL,
    started_at      TIMESTAMP NOT NULL,
    ended_at        TIMESTAMP
);

CREATE TABLE lessons (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id        UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    scheduled_at     TIMESTAMP NOT NULL,
    duration_minutes INT NOT NULL,
    status           VARCHAR(10) NOT NULL DEFAULT 'scheduled'
                         CHECK (status IN ('scheduled', 'completed', 'cancelled', 'missed')),
    notes            TEXT
);

CREATE TABLE payments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id     UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    amount        NUMERIC(10,2) NOT NULL,
    lessons_count INT NOT NULL,
    paid_at       TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS lessons;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS students;
DROP TABLE IF EXISTS tutors;

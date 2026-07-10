-- +goose Up
ALTER TABLE students
  ADD COLUMN username          TEXT,
  ADD COLUMN password_hash     TEXT,
  ADD COLUMN invite_token      UUID,
  ADD COLUMN invite_expires_at TIMESTAMPTZ;

CREATE UNIQUE INDEX students_username_key
  ON students (username) WHERE password_hash IS NOT NULL;
CREATE UNIQUE INDEX students_phone_account_key
  ON students (phone) WHERE password_hash IS NOT NULL;
CREATE UNIQUE INDEX students_invite_token_key
  ON students (invite_token) WHERE invite_token IS NOT NULL;

CREATE TABLE student_refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX student_refresh_tokens_token_idx      ON student_refresh_tokens(token);
CREATE INDEX student_refresh_tokens_student_id_idx ON student_refresh_tokens(student_id);

-- +goose Down
DROP TABLE IF EXISTS student_refresh_tokens;
DROP INDEX IF EXISTS students_invite_token_key;
DROP INDEX IF EXISTS students_phone_account_key;
DROP INDEX IF EXISTS students_username_key;
ALTER TABLE students
  DROP COLUMN invite_expires_at,
  DROP COLUMN invite_token,
  DROP COLUMN password_hash,
  DROP COLUMN username;

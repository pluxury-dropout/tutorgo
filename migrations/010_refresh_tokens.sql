-- +goose Up
CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id   UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX refresh_tokens_token_idx    ON refresh_tokens(token);
CREATE INDEX refresh_tokens_tutor_id_idx ON refresh_tokens(tutor_id);

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;

-- +goose Up
CREATE TABLE pending_registrations (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,     -- один pending на email (upsert)
    password_hash TEXT        NOT NULL,            -- bcrypt пароля
    first_name    TEXT        NOT NULL,
    last_name     TEXT        NOT NULL,
    phone         TEXT        NOT NULL DEFAULT '',
    code_hash     TEXT        NOT NULL,            -- bcrypt 6-значного кода
    attempts      INT         NOT NULL DEFAULT 0,
    resend_at     TIMESTAMPTZ NOT NULL,            -- когда можно повторно отправить код
    expires_at    TIMESTAMPTZ NOT NULL,            -- created_at + 10 мин
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS pending_registrations;

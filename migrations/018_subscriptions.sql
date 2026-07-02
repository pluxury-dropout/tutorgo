-- +goose Up
CREATE TABLE subscriptions (
    tutor_id      UUID PRIMARY KEY REFERENCES tutors(id) ON DELETE CASCADE,
    plan          TEXT,
    period_end    TIMESTAMPTZ,
    grandfathered BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Бэкфилл: все существующие репетиторы — бесплатно навсегда.
INSERT INTO subscriptions (tutor_id, grandfathered)
SELECT id, TRUE FROM tutors
ON CONFLICT (tutor_id) DO NOTHING;

-- +goose Down
DROP TABLE subscriptions;

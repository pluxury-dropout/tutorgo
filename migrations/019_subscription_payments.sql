-- +goose Up
ALTER TABLE subscriptions
  ADD COLUMN card_token   TEXT,
  ADD COLUMN autopay      BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN pending_plan TEXT;

CREATE TABLE subscription_payments (
  id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tutor_id            UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
  provider_payment_id TEXT UNIQUE,
  order_id            TEXT NOT NULL UNIQUE,
  plan                TEXT NOT NULL,
  amount              INTEGER NOT NULL,
  status              TEXT NOT NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE subscription_payments;
ALTER TABLE subscriptions
  DROP COLUMN card_token,
  DROP COLUMN autopay,
  DROP COLUMN pending_plan;

-- +goose Up
-- События репетитора: врач, спортзал, подготовка материалов, пробный урок.
-- Отдельная таблица, а не расширение tasks: у задачи шкала срочности и
-- необязательное время, у события время есть всегда, а срочности нет.
CREATE TABLE events (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id         UUID        NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    title            TEXT        NOT NULL,
    kind             TEXT        NOT NULL DEFAULT 'personal'
                                 CHECK (kind IN ('personal','work','trial')),
    starts_at        TIMESTAMPTZ NOT NULL,
    duration_minutes INT         NOT NULL CHECK (duration_minutes > 0),
    color            TEXT        NOT NULL DEFAULT '',
    location         TEXT        NOT NULL DEFAULT '',
    notes            TEXT        NOT NULL DEFAULT '',

    -- зарезервировано под внешнюю синхронизацию, пока не используется
    external_source  TEXT        NOT NULL DEFAULT '',
    external_id      TEXT        NOT NULL DEFAULT '',
    external_etag    TEXT        NOT NULL DEFAULT '',

    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_events_tutor_starts ON events(tutor_id, starts_at);

-- +goose Down
DROP TABLE IF EXISTS events;

-- +goose Up

CREATE TABLE boards (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id  UUID NOT NULL REFERENCES courses(id) ON DELETE CASCADE,
    tutor_id   UUID NOT NULL REFERENCES tutors(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(course_id)
);

CREATE TABLE board_pages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    title      VARCHAR(100) NOT NULL DEFAULT 'Страница 1',
    snapshot   JSONB,
    position   INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE board_assets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    file_path  VARCHAR NOT NULL,
    mime_type  VARCHAR(50) NOT NULL,
    size_bytes INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE board_invites (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(board_id)
);

-- +goose Down
DROP TABLE IF EXISTS board_invites;
DROP TABLE IF EXISTS board_assets;
DROP TABLE IF EXISTS board_pages;
DROP TABLE IF EXISTS boards;

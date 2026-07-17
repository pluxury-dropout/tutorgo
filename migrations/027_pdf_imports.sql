-- +goose Up

CREATE TABLE board_pdf_imports (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id   UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    page_id    UUID NOT NULL REFERENCES board_pages(id) ON DELETE CASCADE,
    tutor_id   UUID NOT NULL REFERENCES tutors(id),
    s3_key     VARCHAR NOT NULL,
    status     VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending | rendering | done | failed
    pages      JSONB NOT NULL DEFAULT '[]',
    error      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Чистка протухших pending и добор незавершённых rendering при старте воркера.
CREATE INDEX idx_pdf_imports_status ON board_pdf_imports(status, created_at);

-- +goose Down
DROP TABLE IF EXISTS board_pdf_imports;

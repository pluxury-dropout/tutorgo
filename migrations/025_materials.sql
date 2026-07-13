-- +goose Up
CREATE TABLE materials (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tutor_id   UUID NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    parent_id  UUID REFERENCES materials(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('folder', 'file')),
    name       TEXT NOT NULL,
    file_path  TEXT,
    mime_type  TEXT,
    size_bytes INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (kind = 'folder' OR file_path IS NOT NULL)
);
CREATE INDEX idx_materials_tutor_parent ON materials (tutor_id, parent_id);

-- +goose Down
DROP TABLE IF EXISTS materials;

package repository

import (
	"context"

	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type MaterialRepository interface {
	ListByParent(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error)
	GetByID(ctx context.Context, id string) (models.Material, error)
	CreateFolder(ctx context.Context, tutorID, name string, parentID *string) (models.Material, error)
	CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error)
	HasChildren(ctx context.Context, id string) (bool, error)
	Delete(ctx context.Context, id, tutorID string) error
}

type materialRepository struct {
	conn *pgxpool.Pool
}

func NewMaterialRepository(conn *pgxpool.Pool) MaterialRepository {
	return &materialRepository{conn: conn}
}

// materialCols — общий список колонок, чтобы SELECT'ы и RETURNING не разъезжались
// со сканом. Nullable-поля схлопываем в нули, чтобы модель обходилась без указателей.
const materialCols = `id, tutor_id, parent_id, kind, name,
	COALESCE(file_path, ''), COALESCE(mime_type, ''), COALESCE(size_bytes, 0), created_at`

func (r *materialRepository) ListByParent(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error) {
	// parent_id IS NOT DISTINCT FROM $2 — один запрос и для корня (NULL), и для
	// папки: обычное `=` с NULL всегда даёт NULL, т.е. пустой список.
	rows, err := r.conn.Query(ctx,
		`SELECT `+materialCols+` FROM materials
		 WHERE tutor_id = $1 AND parent_id IS NOT DISTINCT FROM $2
		 ORDER BY kind = 'file', name`,
		tutorID, parentID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.Material{}
	for rows.Next() {
		var m models.Material
		if err := rows.Scan(&m.ID, &m.TutorID, &m.ParentID, &m.Kind, &m.Name,
			&m.FilePath, &m.MimeType, &m.SizeBytes, &m.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

func (r *materialRepository) GetByID(ctx context.Context, id string) (models.Material, error) {
	var m models.Material
	err := r.conn.QueryRow(ctx,
		`SELECT `+materialCols+` FROM materials WHERE id = $1`, id,
	).Scan(&m.ID, &m.TutorID, &m.ParentID, &m.Kind, &m.Name,
		&m.FilePath, &m.MimeType, &m.SizeBytes, &m.CreatedAt)
	return m, err
}

func (r *materialRepository) CreateFolder(ctx context.Context, tutorID, name string, parentID *string) (models.Material, error) {
	var m models.Material
	err := r.conn.QueryRow(ctx,
		`INSERT INTO materials (tutor_id, parent_id, kind, name)
		 VALUES ($1, $2, 'folder', $3)
		 RETURNING `+materialCols,
		tutorID, parentID, name,
	).Scan(&m.ID, &m.TutorID, &m.ParentID, &m.Kind, &m.Name,
		&m.FilePath, &m.MimeType, &m.SizeBytes, &m.CreatedAt)
	return m, err
}

func (r *materialRepository) CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error) {
	var m models.Material
	err := r.conn.QueryRow(ctx,
		`INSERT INTO materials (tutor_id, parent_id, kind, name, file_path, mime_type, size_bytes)
		 VALUES ($1, $2, 'file', $3, $4, $5, $6)
		 RETURNING `+materialCols,
		tutorID, parentID, name, filePath, mimeType, sizeBytes,
	).Scan(&m.ID, &m.TutorID, &m.ParentID, &m.Kind, &m.Name,
		&m.FilePath, &m.MimeType, &m.SizeBytes, &m.CreatedAt)
	return m, err
}

func (r *materialRepository) HasChildren(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM materials WHERE parent_id = $1)`, id,
	).Scan(&exists)
	return exists, err
}

func (r *materialRepository) Delete(ctx context.Context, id, tutorID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM materials WHERE id = $1 AND tutor_id = $2`, id, tutorID)
	return err
}

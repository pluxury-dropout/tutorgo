package repository

import (
	"context"
	"encoding/json"
	"tutorgo/models"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WhiteboardRepository interface {
	CourseBelongsToTutor(ctx context.Context, courseID, tutorID string) (bool, error)
	GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.Board, error)
	GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.Board, error)
	GetBoardByID(ctx context.Context, boardID string) (models.Board, error)
	GetBoardByInvite(ctx context.Context, inviteID string) (models.Board, error)
	GetPagesByBoard(ctx context.Context, boardID string) ([]models.BoardPage, error)
	GetPageByID(ctx context.Context, pageID string) (models.BoardPage, error)
	CreatePage(ctx context.Context, boardID, title string, position int) (models.BoardPage, error)
	UpdatePage(ctx context.Context, pageID string, req models.UpdateBoardPageRequest) (models.BoardPage, error)
	SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error
	DeletePage(ctx context.Context, pageID, boardID string) error
	CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error)
	DeleteInvite(ctx context.Context, boardID string) error
	GetInviteByBoard(ctx context.Context, boardID string) (models.BoardInvite, error)
	CreateAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error)
	GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error)
	GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error)
}

type whiteboardRepository struct {
	conn *pgxpool.Pool
}

func NewWhiteboardRepository(conn *pgxpool.Pool) WhiteboardRepository {
	return &whiteboardRepository{conn: conn}
}

func (r *whiteboardRepository) CourseBelongsToTutor(ctx context.Context, courseID, tutorID string) (bool, error) {
	var exists bool
	err := r.conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM courses WHERE id = $1 AND tutor_id = $2)`,
		courseID, tutorID,
	).Scan(&exists)
	return exists, err
}

func (r *whiteboardRepository) GetBoardByID(ctx context.Context, boardID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`SELECT id, COALESCE(course_id::text, ''), tutor_id, created_at FROM boards WHERE id = $1`,
		boardID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}

func (r *whiteboardRepository) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`INSERT INTO boards (course_id, tutor_id)
         VALUES ($1, $2)
         ON CONFLICT (course_id) DO UPDATE SET course_id = EXCLUDED.course_id
         RETURNING id, COALESCE(course_id::text, ''), tutor_id, created_at`,
		courseID, tutorID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}

// GetOrCreateTrialBoard — доска для пробных уроков: одна на препода, вне курсов.
// ON CONFLICT целится в частичный индекс idx_boards_trial (tutor_id) WHERE
// course_id IS NULL — предикат в конфликте обязателен, иначе Postgres не поймёт,
// какой индекс имеется в виду.
func (r *whiteboardRepository) GetOrCreateTrialBoard(ctx context.Context, tutorID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`INSERT INTO boards (course_id, tutor_id)
         VALUES (NULL, $1)
         ON CONFLICT (tutor_id) WHERE course_id IS NULL
         DO UPDATE SET tutor_id = EXCLUDED.tutor_id
         RETURNING id, COALESCE(course_id::text, ''), tutor_id, created_at`,
		tutorID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}

func (r *whiteboardRepository) GetBoardByInvite(ctx context.Context, inviteID string) (models.Board, error) {
	var b models.Board
	err := r.conn.QueryRow(ctx,
		`SELECT bo.id, COALESCE(bo.course_id::text, ''), bo.tutor_id, bo.created_at
         FROM boards bo
         JOIN board_invites bi ON bi.board_id = bo.id
         WHERE bi.id = $1`,
		inviteID,
	).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
	return b, err
}

func (r *whiteboardRepository) GetPagesByBoard(ctx context.Context, boardID string) ([]models.BoardPage, error) {
	rows, err := r.conn.Query(ctx,
		`SELECT id, board_id, title, snapshot, position, created_at, updated_at
         FROM board_pages WHERE board_id = $1 ORDER BY position ASC, created_at ASC`,
		boardID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pages []models.BoardPage
	for rows.Next() {
		var p models.BoardPage
		if err := rows.Scan(&p.ID, &p.BoardID, &p.Title, &p.Snapshot, &p.Position, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (r *whiteboardRepository) GetPageByID(ctx context.Context, pageID string) (models.BoardPage, error) {
	var p models.BoardPage
	err := r.conn.QueryRow(ctx,
		`SELECT id, board_id, title, snapshot, position, created_at, updated_at
         FROM board_pages WHERE id = $1`,
		pageID,
	).Scan(&p.ID, &p.BoardID, &p.Title, &p.Snapshot, &p.Position, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *whiteboardRepository) CreatePage(ctx context.Context, boardID, title string, position int) (models.BoardPage, error) {
	var p models.BoardPage
	err := r.conn.QueryRow(ctx,
		`INSERT INTO board_pages (board_id, title, position)
         VALUES ($1, $2, $3)
         RETURNING id, board_id, title, snapshot, position, created_at, updated_at`,
		boardID, title, position,
	).Scan(&p.ID, &p.BoardID, &p.Title, &p.Snapshot, &p.Position, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *whiteboardRepository) UpdatePage(ctx context.Context, pageID string, req models.UpdateBoardPageRequest) (models.BoardPage, error) {
	var p models.BoardPage
	err := r.conn.QueryRow(ctx,
		`UPDATE board_pages
         SET title    = COALESCE($2, title),
             position = COALESCE($3, position),
             updated_at = now()
         WHERE id = $1
         RETURNING id, board_id, title, snapshot, position, created_at, updated_at`,
		pageID, req.Title, req.Position,
	).Scan(&p.ID, &p.BoardID, &p.Title, &p.Snapshot, &p.Position, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *whiteboardRepository) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE board_pages SET snapshot = $2, updated_at = now() WHERE id = $1`,
		pageID, snapshot,
	)
	return err
}

func (r *whiteboardRepository) DeletePage(ctx context.Context, pageID, boardID string) error {
	_, err := r.conn.Exec(ctx,
		`DELETE FROM board_pages WHERE id = $1 AND board_id = $2`,
		pageID, boardID,
	)
	return err
}

func (r *whiteboardRepository) CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error) {
	// Atomically create the single invite per board, or return the existing one
	// unchanged. The invite is a stable, permanent link — repeated calls must
	// not rotate its id (ON CONFLICT DO UPDATE is a no-op update, kept only so
	// RETURNING still yields a row).
	var inv models.BoardInvite
	err := r.conn.QueryRow(ctx,
		`INSERT INTO board_invites (board_id) VALUES ($1)
         ON CONFLICT (board_id) DO UPDATE SET board_id = EXCLUDED.board_id
         RETURNING id, board_id, created_at`,
		boardID,
	).Scan(&inv.ID, &inv.BoardID, &inv.CreatedAt)
	return inv, err
}

func (r *whiteboardRepository) DeleteInvite(ctx context.Context, boardID string) error {
	_, err := r.conn.Exec(ctx, `DELETE FROM board_invites WHERE board_id = $1`, boardID)
	return err
}

func (r *whiteboardRepository) GetInviteByBoard(ctx context.Context, boardID string) (models.BoardInvite, error) {
	var inv models.BoardInvite
	err := r.conn.QueryRow(ctx,
		`SELECT id, board_id, created_at FROM board_invites WHERE board_id = $1`,
		boardID,
	).Scan(&inv.ID, &inv.BoardID, &inv.CreatedAt)
	return inv, err
}

func (r *whiteboardRepository) CreateAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
	var a models.BoardAsset
	err := r.conn.QueryRow(ctx,
		`INSERT INTO board_assets (board_id, file_path, mime_type, size_bytes)
         VALUES ($1, $2, $3, $4)
         RETURNING id, board_id, file_path, mime_type, size_bytes, created_at`,
		boardID, filePath, mimeType, sizeBytes,
	).Scan(&a.ID, &a.BoardID, &a.FilePath, &a.MimeType, &a.SizeBytes, &a.CreatedAt)
	return a, err
}

func (r *whiteboardRepository) GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error) {
	var a models.BoardAsset
	err := r.conn.QueryRow(ctx,
		`SELECT id, board_id, file_path, mime_type, size_bytes, created_at
         FROM board_assets WHERE id = $1`,
		assetID,
	).Scan(&a.ID, &a.BoardID, &a.FilePath, &a.MimeType, &a.SizeBytes, &a.CreatedAt)
	return a, err
}

func (r *whiteboardRepository) GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error) {
	var snap json.RawMessage
	err := r.conn.QueryRow(ctx,
		`SELECT snapshot FROM board_pages WHERE id = $1`, pageID,
	).Scan(&snap)
	return snap, err
}

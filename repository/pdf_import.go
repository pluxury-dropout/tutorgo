package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tutorgo/models"
)

var (
	// ErrImportAlreadyStarted — Start вызван повторно (status уже не pending).
	ErrImportAlreadyStarted = errors.New("import already started")
	// ErrBadPageRange — запрошенный диапазон страниц невалиден или выходит за границы документа.
	ErrBadPageRange = errors.New("bad page range")
)

type PdfImportRepository interface {
	// Create пишет запись pending со всеми страницами документа (AssetID пустые).
	Create(ctx context.Context, boardID, pageID, tutorID, s3Key string, pages []models.PdfImportPage) (string, error)
	GetByID(ctx context.Context, id string) (models.PdfImport, error)
	// Start в одной транзакции: board_assets для страниц [from..to], pages с
	// заполненными AssetID, статус rendering и enqueue(tx) — вставка River-джобы.
	// Возвращает страницы диапазона.
	Start(ctx context.Context, id string, from, to int, enqueue func(pgx.Tx) error) ([]models.PdfImportPage, error)
	MarkPageDone(ctx context.Context, id string, n int) error
	SetStatus(ctx context.Context, id, status string, errMsg *string) error
	// DeleteStalePending удаляет pending старше 24ч, возвращает их s3-ключи
	// (воркер удалит объекты из S3 best-effort).
	DeleteStalePending(ctx context.Context) ([]string, error)
	BoardBelongsToTutor(ctx context.Context, boardID, tutorID string) (bool, error)
	PageBelongsToBoard(ctx context.Context, pageID, boardID string) (bool, error)
}

type pdfImportRepository struct {
	conn *pgxpool.Pool
}

func NewPdfImportRepository(conn *pgxpool.Pool) PdfImportRepository {
	return &pdfImportRepository{conn: conn}
}

func (r *pdfImportRepository) Create(ctx context.Context, boardID, pageID, tutorID, s3Key string, pages []models.PdfImportPage) (string, error) {
	pagesJSON, err := json.Marshal(pages)
	if err != nil {
		return "", err
	}
	var id string
	err = r.conn.QueryRow(ctx,
		`INSERT INTO board_pdf_imports (board_id, page_id, tutor_id, s3_key, pages)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		boardID, pageID, tutorID, s3Key, pagesJSON,
	).Scan(&id)
	return id, err
}

func (r *pdfImportRepository) GetByID(ctx context.Context, id string) (models.PdfImport, error) {
	var imp models.PdfImport
	var pagesJSON []byte
	err := r.conn.QueryRow(ctx,
		`SELECT id, board_id, page_id, tutor_id, s3_key, status, pages, error
		 FROM board_pdf_imports WHERE id = $1`, id,
	).Scan(&imp.ID, &imp.BoardID, &imp.PageID, &imp.TutorID, &imp.S3Key, &imp.Status, &pagesJSON, &imp.Error)
	if err != nil {
		return imp, err
	}
	err = json.Unmarshal(pagesJSON, &imp.Pages)
	return imp, err
}

func (r *pdfImportRepository) Start(ctx context.Context, id string, from, to int, enqueue func(pgx.Tx) error) ([]models.PdfImportPage, error) {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // безвредно после Commit

	// FOR UPDATE: два параллельных start одного импорта не должны наплодить ассетов.
	var pagesJSON []byte
	var status, boardID string
	err = tx.QueryRow(ctx,
		`SELECT pages, status, board_id FROM board_pdf_imports WHERE id = $1 FOR UPDATE`, id,
	).Scan(&pagesJSON, &status, &boardID)
	if err != nil {
		return nil, err
	}
	if status != "pending" {
		return nil, fmt.Errorf("import %s already started (status %s): %w", id, status, ErrImportAlreadyStarted)
	}
	var all []models.PdfImportPage
	if err := json.Unmarshal(pagesJSON, &all); err != nil {
		return nil, err
	}
	if from < 1 || to > len(all) || from > to {
		return nil, fmt.Errorf("range %d-%d out of 1-%d: %w", from, to, len(all), ErrBadPageRange)
	}

	selected := all[from-1 : to] // страницы 1-индексированы и лежат по порядку
	for i := range selected {
		key := models.PdfPageAssetKey(id, selected[i].N)
		// size_bytes=0 — реальный размер появится после рендера, он никому не нужен.
		err = tx.QueryRow(ctx,
			`INSERT INTO board_assets (board_id, file_path, mime_type, size_bytes)
			 VALUES ($1, $2, 'image/jpeg', 0) RETURNING id`,
			boardID, key,
		).Scan(&selected[i].AssetID)
		if err != nil {
			return nil, err
		}
	}

	newPages, err := json.Marshal(selected)
	if err != nil {
		return nil, err
	}
	// В pages остаётся только выбранный диапазон: остальные страницы импорту
	// больше не принадлежат, а воркер работает ровно по этому списку.
	if _, err = tx.Exec(ctx,
		`UPDATE board_pdf_imports SET pages = $2, status = 'rendering', updated_at = now() WHERE id = $1`,
		id, newPages,
	); err != nil {
		return nil, err
	}
	if err = enqueue(tx); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return selected, nil
}

func (r *pdfImportRepository) MarkPageDone(ctx context.Context, id string, n int) error {
	// jsonb-обход: помечаем done у элемента массива с данным n.
	_, err := r.conn.Exec(ctx,
		`UPDATE board_pdf_imports SET pages = (
		    SELECT jsonb_agg(CASE WHEN (p->>'n')::int = $2
		                          THEN jsonb_set(p, '{done}', 'true') ELSE p END)
		    FROM jsonb_array_elements(pages) p
		 ), updated_at = now() WHERE id = $1`,
		id, n,
	)
	return err
}

func (r *pdfImportRepository) SetStatus(ctx context.Context, id, status string, errMsg *string) error {
	_, err := r.conn.Exec(ctx,
		`UPDATE board_pdf_imports SET status = $2, error = $3, updated_at = now() WHERE id = $1`,
		id, status, errMsg,
	)
	return err
}

func (r *pdfImportRepository) DeleteStalePending(ctx context.Context) ([]string, error) {
	rows, err := r.conn.Query(ctx,
		`DELETE FROM board_pdf_imports
		 WHERE status = 'pending' AND created_at < now() - interval '24 hours'
		 RETURNING s3_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *pdfImportRepository) BoardBelongsToTutor(ctx context.Context, boardID, tutorID string) (bool, error) {
	var ok bool
	err := r.conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM boards WHERE id = $1 AND tutor_id = $2)`,
		boardID, tutorID,
	).Scan(&ok)
	return ok, err
}

func (r *pdfImportRepository) PageBelongsToBoard(ctx context.Context, pageID, boardID string) (bool, error) {
	var ok bool
	err := r.conn.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM board_pages WHERE id = $1 AND board_id = $2)`,
		pageID, boardID,
	).Scan(&ok)
	return ok, err
}

// NotifyBoardEvent шлёт событие всем API-инстансам через pg_notify. Payload
// NOTIFY ограничен 8000 байтами — наши события на порядок меньше.
func NotifyBoardEvent(ctx context.Context, conn *pgxpool.Pool, ev models.BoardEvent) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = conn.Exec(ctx, `SELECT pg_notify('board_events', $1)`, string(payload))
	return err
}

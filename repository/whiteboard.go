package repository

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"time"
	"tutorgo/models"

	"github.com/jackc/pgx/v5"
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

	// Поэлементная модель (миграция 029).
	MergeElements(ctx context.Context, pageID string, els []models.BoardElement) error
	MergeFiles(ctx context.Context, pageID string, files json.RawMessage) error
	GetPageState(ctx context.Context, pageID string) (state json.RawMessage, migrated bool, err error)
	ImportSnapshotToElements(ctx context.Context, pageID string) error
	DeleteOldTombstones(ctx context.Context) (int64, error)
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
		`SELECT id, board_id, title, position, created_at, updated_at
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
		if err := rows.Scan(&p.ID, &p.BoardID, &p.Title, &p.Position, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pages = append(pages, p)
	}
	return pages, rows.Err()
}

func (r *whiteboardRepository) GetPageByID(ctx context.Context, pageID string) (models.BoardPage, error) {
	var p models.BoardPage
	err := r.conn.QueryRow(ctx,
		`SELECT id, board_id, title, position, created_at, updated_at
         FROM board_pages WHERE id = $1`,
		pageID,
	).Scan(&p.ID, &p.BoardID, &p.Title, &p.Position, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *whiteboardRepository) CreatePage(ctx context.Context, boardID, title string, position int) (models.BoardPage, error) {
	var p models.BoardPage
	err := r.conn.QueryRow(ctx,
		`INSERT INTO board_pages (board_id, title, position)
         VALUES ($1, $2, $3)
         RETURNING id, board_id, title, position, created_at, updated_at`,
		boardID, title, position,
	).Scan(&p.ID, &p.BoardID, &p.Title, &p.Position, &p.CreatedAt, &p.UpdatedAt)
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
         RETURNING id, board_id, title, position, created_at, updated_at`,
		pageID, req.Title, req.Position,
	).Scan(&p.ID, &p.BoardID, &p.Title, &p.Position, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *whiteboardRepository) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
	compressed, err := gzipSnapshot(snapshot)
	if err != nil {
		return err
	}
	_, err = r.conn.Exec(ctx,
		`UPDATE board_pages SET snapshot = $2, updated_at = now() WHERE id = $1`,
		pageID, compressed,
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

// mergeElementSQL — авторитетное правило слияния Excalidraw, выраженное в
// WHERE-клаузе UPSERT'а: без блокировок и без чтения перед записью. Побеждает
// больший version; при равных версиях — МЕНЬШИЙ nonce (см. BoardElement.Beats,
// там же обоснование контринтуитивного знака).
//
// Два инстанса могут писать один элемент одновременно — результат детерминирован
// средствами Postgres. Проигравшая запись просто не выполняется.
const mergeElementSQL = `
INSERT INTO board_elements (page_id, element_id, version, nonce, data)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (page_id, element_id) DO UPDATE
   SET version    = EXCLUDED.version,
       nonce      = EXCLUDED.nonce,
       data       = EXCLUDED.data,
       updated_at = now()
 WHERE board_elements.version < EXCLUDED.version
    OR (board_elements.version = EXCLUDED.version
        AND board_elements.nonce > EXCLUDED.nonce)`

func (r *whiteboardRepository) MergeElements(ctx context.Context, pageID string, els []models.BoardElement) error {
	if len(els) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range els {
		batch.Queue(mergeElementSQL, pageID, e.ID, e.Version, e.Nonce, e.Data)
	}
	// Close возвращает первую ошибку пачки и дочитывает остальные результаты —
	// разбирать их поштучно незачем: сбой тут означает недоступную БД, а не
	// проблему конкретного элемента.
	return r.conn.SendBatch(ctx, batch).Close()
}

// MergeFiles дописывает указатели на файлы. Записи иммутабельны (новая картинка
// получает новый fileId), поэтому конфликта нет и слияние — это `||`.
func (r *whiteboardRepository) MergeFiles(ctx context.Context, pageID string, files json.RawMessage) error {
	if len(files) == 0 {
		return nil
	}
	_, err := r.conn.Exec(ctx,
		`UPDATE board_pages SET files = files || $2 WHERE id = $1`,
		pageID, files,
	)
	return err
}

// pageStateSQL собирает сид целиком силами Postgres: на выходе готовое
// {"elements":[…],"files":{…}}, сборки на стороне Go нет.
//
// Порядок элементов — по фракционному индексу Excalidraw (`index`,
// лексикографически сортируемая строка): это и есть z-order сцены. Элементы без
// индекса (Excalidraw допускает null) идут первыми и получат индекс при загрузке;
// element_id вторым ключом делает порядок детерминированным.
const pageStateSQL = `
SELECT jsonb_build_object(
           'elements', COALESCE(
               jsonb_agg(e.data ORDER BY e.data->>'index' ASC NULLS FIRST, e.element_id)
                   FILTER (WHERE e.element_id IS NOT NULL),
               '[]'::jsonb),
           'files', p.files),
       p.elements_migrated_at IS NOT NULL
  FROM board_pages p
  LEFT JOIN board_elements e ON e.page_id = p.id
 WHERE p.id = $1
 GROUP BY p.id`

func (r *whiteboardRepository) GetPageState(ctx context.Context, pageID string) (json.RawMessage, bool, error) {
	var state []byte
	var migrated bool
	if err := r.conn.QueryRow(ctx, pageStateSQL, pageID).Scan(&state, &migrated); err != nil {
		return nil, false, err
	}
	return state, migrated, nil
}

// ImportSnapshotToElements переносит страницу со старой модели (один BLOB) на
// поэлементную. Идемпотентна: FOR UPDATE сериализует два одновременных
// подключения к одной странице, и второе увидит выставленный флаг.
//
// Признак «уже мигрировали» — именно флаг, а не наличие строк: через сутки
// чистка tombstones обнулит полностью очищенную доску, и импорт по признаку
// «нет строк» воскресил бы старый снапшот.
func (r *whiteboardRepository) ImportSnapshotToElements(ctx context.Context, pageID string) error {
	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // после Commit — no-op

	var stored []byte
	var migratedAt *time.Time
	err = tx.QueryRow(ctx,
		`SELECT snapshot, elements_migrated_at FROM board_pages WHERE id = $1 FOR UPDATE`,
		pageID,
	).Scan(&stored, &migratedAt)
	if err != nil {
		return err
	}
	if migratedAt != nil {
		return nil // успели параллельно
	}

	snapshot, err := gunzipSnapshot(stored)
	if err != nil {
		// Битый BLOB не должен запирать доску навсегда: помечаем мигрированной
		// и открываем пустой. Содержимое всё равно нечитаемо.
		snapshot = nil
	}
	var snap struct {
		Elements []json.RawMessage `json:"elements"`
		Files    json.RawMessage   `json:"files"`
	}
	if len(snapshot) > 0 {
		_ = json.Unmarshal(snapshot, &snap) // мусор/старый tldraw-формат → пустая доска
	}

	els, _ := models.ParseBoardElements(snap.Elements)
	if len(els) > 0 {
		batch := &pgx.Batch{}
		for _, e := range els {
			batch.Queue(
				`INSERT INTO board_elements (page_id, element_id, version, nonce, data)
                 VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
				pageID, e.ID, e.Version, e.Nonce, e.Data)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}
	}
	if len(snap.Files) == 0 {
		snap.Files = json.RawMessage(`{}`)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE board_pages SET files = files || $2, elements_migrated_at = now() WHERE id = $1`,
		pageID, snap.Files,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteOldTombstones убирает удалённые элементы старше суток. Свежие держим:
// без них пир, пропустивший удаление офлайн, воскресит элемент через reconcile.
// За сутки такой вкладке не дожить до реконнекта — воскрешать некому.
func (r *whiteboardRepository) DeleteOldTombstones(ctx context.Context) (int64, error) {
	tag, err := r.conn.Exec(ctx,
		`DELETE FROM board_elements
          WHERE updated_at < now() - interval '24 hours'
            AND data->>'isDeleted' = 'true'`)
	return tag.RowsAffected(), err
}

// Снапшоты хранятся gzip-ом в bytea (миграция 028). Записи, оставшиеся от jsonb,
// лежат сырым JSON — различаем по gzip-magic, чтобы доски, которые с миграции
// никто не открывал, продолжали читаться.
func isGzip(b []byte) bool {
	return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b
}

func gzipSnapshot(snapshot json.RawMessage) ([]byte, error) {
	var buf bytes.Buffer
	// BestSpeed, а не BestCompression: снапшот пишется раз в несколько секунд на
	// каждой активной доске, и на этих данных (JSON с повторяющимися ключами)
	// быстрый уровень отстаёт от максимального на считанные проценты.
	zw, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(snapshot); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func gunzipSnapshot(stored []byte) (json.RawMessage, error) {
	if !isGzip(stored) {
		return stored, nil // NULL или досжатая запись — отдаём как есть
	}
	zr, err := gzip.NewReader(bytes.NewReader(stored))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

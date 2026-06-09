# Whiteboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать интерактивную доску с tldraw, привязанную к курсу, с real-time коллаборацией через Go WebSocket Hub и постоянным хранением в PostgreSQL.

**Architecture:** tldraw-компонент в браузере отправляет diff-патчи по WebSocket. Go-бэкенд держит одну горутину-hub на активную страницу доски: принимает патчи, рассылает всем участникам, сохраняет snapshot в БД с дебаунсом 2с. Ученик заходит по guest-ссылке (UUID из `board_invites`).

**Tech Stack:** Go (gorilla/websocket, bep/debounce), PostgreSQL (JSONB snapshot), Next.js 14 (App Router), @tldraw/tldraw v2, @tanstack/react-query, pdfjs-dist

---

## File Map

### Backend (новые файлы)
- `migrations/014_whiteboard.sql` — создание таблиц boards/board_pages/board_assets/board_invites
- `models/whiteboard.go` — Go-структуры Board, BoardPage, BoardAsset, BoardInvite + request/response типы
- `repository/whiteboard.go` — интерфейс + реализация CRUD
- `service/whiteboard.go` — бизнес-логика (GetOrCreate, CreatePage, Invite и т.д.)
- `service/whiteboard_test.go` — юнит-тесты сервиса
- `handlers/whiteboard.go` — REST-хендлеры
- `handlers/whiteboard_ws.go` — WebSocket Hub (HubManager, Hub, Client)

### Backend (модификации)
- `router/router.go` — регистрация репозитория, сервиса, хендлеров, маршрутов

### Frontend (новые файлы)
- `frontend/src/lib/api/whiteboard.ts` — API-клиент
- `frontend/src/lib/hooks/useWhiteboard.ts` — React Query хуки
- `frontend/src/components/whiteboard/useWhiteboardSync.ts` — WS-хук (tldraw ↔ WebSocket)
- `frontend/src/components/whiteboard/TldrawCanvas.tsx` — обёртка над <Tldraw />
- `frontend/src/components/whiteboard/PageSidebar.tsx` — список страниц
- `frontend/src/components/whiteboard/BoardToolbar.tsx` — кнопки (пригласить, загрузить PDF)
- `frontend/src/app/(dashboard)/boards/[courseId]/page.tsx` — страница доски репетитора
- `frontend/src/app/board/join/[token]/page.tsx` — страница входа ученика

### Frontend (модификации)
- `frontend/src/types/api.ts` — добавить типы Board, BoardPage, BoardInvite, BoardAsset

---

## Task 1: DB Migration

**Files:**
- Create: `migrations/014_whiteboard.sql`

- [ ] **Step 1: Создать файл миграции**

```sql
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
```

- [ ] **Step 2: Применить миграцию**

```bash
goose -dir migrations postgres "$DB_URL" up
```

Ожидается: `OK   014_whiteboard.sql`

- [ ] **Step 3: Коммит**

```bash
git add migrations/014_whiteboard.sql
git commit -m "feat: add whiteboard DB migration (boards, pages, assets, invites)"
```

---

## Task 2: Go Models

**Files:**
- Create: `models/whiteboard.go`

- [ ] **Step 1: Создать модели**

```go
package models

import (
    "encoding/json"
    "time"
)

type Board struct {
    ID        string    `json:"id"`
    CourseID  string    `json:"course_id"`
    TutorID   string    `json:"tutor_id"`
    CreatedAt time.Time `json:"created_at"`
}

type BoardPage struct {
    ID        string          `json:"id"`
    BoardID   string          `json:"board_id"`
    Title     string          `json:"title"`
    Snapshot  json.RawMessage `json:"snapshot,omitempty"`
    Position  int             `json:"position"`
    CreatedAt time.Time       `json:"created_at"`
    UpdatedAt time.Time       `json:"updated_at"`
}

type BoardAsset struct {
    ID        string    `json:"id"`
    BoardID   string    `json:"board_id"`
    FilePath  string    `json:"file_path"`
    MimeType  string    `json:"mime_type"`
    SizeBytes int       `json:"size_bytes"`
    CreatedAt time.Time `json:"created_at"`
}

type BoardInvite struct {
    ID        string    `json:"id"`
    BoardID   string    `json:"board_id"`
    CreatedAt time.Time `json:"created_at"`
}

type CreateBoardPageRequest struct {
    Title string `json:"title" validate:"required,min=1,max=100"`
}

type UpdateBoardPageRequest struct {
    Title    *string `json:"title"    validate:"omitempty,min=1,max=100"`
    Position *int    `json:"position" validate:"omitempty,min=0"`
}

type BoardWithPages struct {
    Board
    Pages []BoardPage `json:"pages"`
}

type BoardAssetResponse struct {
    ID  string `json:"id"`
    URL string `json:"url"`
}
```

- [ ] **Step 2: Убедиться что компилируется**

```bash
go build ./...
```

Ожидается: выход без ошибок

- [ ] **Step 3: Коммит**

```bash
git add models/whiteboard.go
git commit -m "feat: add whiteboard Go models"
```

---

## Task 3: Repository

**Files:**
- Create: `repository/whiteboard.go`

- [ ] **Step 1: Создать репозиторий**

```go
package repository

import (
    "context"
    "encoding/json"
    "tutorgo/models"

    "github.com/jackc/pgx/v5/pgxpool"
)

type WhiteboardRepository interface {
    GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.Board, error)
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
}

type whiteboardRepository struct {
    conn *pgxpool.Pool
}

func NewWhiteboardRepository(conn *pgxpool.Pool) WhiteboardRepository {
    return &whiteboardRepository{conn: conn}
}

func (r *whiteboardRepository) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.Board, error) {
    var b models.Board
    err := r.conn.QueryRow(ctx,
        `INSERT INTO boards (course_id, tutor_id)
         VALUES ($1, $2)
         ON CONFLICT (course_id) DO UPDATE SET course_id = EXCLUDED.course_id
         RETURNING id, course_id, tutor_id, created_at`,
        courseID, tutorID,
    ).Scan(&b.ID, &b.CourseID, &b.TutorID, &b.CreatedAt)
    return b, err
}

func (r *whiteboardRepository) GetBoardByInvite(ctx context.Context, inviteID string) (models.Board, error) {
    var b models.Board
    err := r.conn.QueryRow(ctx,
        `SELECT bo.id, bo.course_id, bo.tutor_id, bo.created_at
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
    // Удаляем предыдущий инвайт (один активный на доску)
    _, _ = r.conn.Exec(ctx, `DELETE FROM board_invites WHERE board_id = $1`, boardID)
    var inv models.BoardInvite
    err := r.conn.QueryRow(ctx,
        `INSERT INTO board_invites (board_id) VALUES ($1) RETURNING id, board_id, created_at`,
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
```

- [ ] **Step 2: Убедиться что компилируется**

```bash
go build ./...
```

- [ ] **Step 3: Коммит**

```bash
git add repository/whiteboard.go
git commit -m "feat: add whiteboard repository"
```

---

## Task 4: Service + Tests

**Files:**
- Create: `service/whiteboard.go`
- Create: `service/whiteboard_test.go`

- [ ] **Step 1: Написать тесты**

```go
package service_test

import (
    "context"
    "encoding/json"
    "testing"
    "tutorgo/models"
    "tutorgo/service"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

type mockWhiteboardRepo struct{ mock.Mock }

func (m *mockWhiteboardRepo) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.Board, error) {
    args := m.Called(ctx, courseID, tutorID)
    return args.Get(0).(models.Board), args.Error(1)
}
func (m *mockWhiteboardRepo) GetBoardByInvite(ctx context.Context, inviteID string) (models.Board, error) {
    args := m.Called(ctx, inviteID)
    return args.Get(0).(models.Board), args.Error(1)
}
func (m *mockWhiteboardRepo) GetPagesByBoard(ctx context.Context, boardID string) ([]models.BoardPage, error) {
    args := m.Called(ctx, boardID)
    return args.Get(0).([]models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) GetPageByID(ctx context.Context, pageID string) (models.BoardPage, error) {
    args := m.Called(ctx, pageID)
    return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) CreatePage(ctx context.Context, boardID, title string, position int) (models.BoardPage, error) {
    args := m.Called(ctx, boardID, title, position)
    return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) UpdatePage(ctx context.Context, pageID string, req models.UpdateBoardPageRequest) (models.BoardPage, error) {
    args := m.Called(ctx, pageID, req)
    return args.Get(0).(models.BoardPage), args.Error(1)
}
func (m *mockWhiteboardRepo) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
    return m.Called(ctx, pageID, snapshot).Error(0)
}
func (m *mockWhiteboardRepo) DeletePage(ctx context.Context, pageID, boardID string) error {
    return m.Called(ctx, pageID, boardID).Error(0)
}
func (m *mockWhiteboardRepo) CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error) {
    args := m.Called(ctx, boardID)
    return args.Get(0).(models.BoardInvite), args.Error(1)
}
func (m *mockWhiteboardRepo) DeleteInvite(ctx context.Context, boardID string) error {
    return m.Called(ctx, boardID).Error(0)
}
func (m *mockWhiteboardRepo) GetInviteByBoard(ctx context.Context, boardID string) (models.BoardInvite, error) {
    args := m.Called(ctx, boardID)
    return args.Get(0).(models.BoardInvite), args.Error(1)
}
func (m *mockWhiteboardRepo) CreateAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
    args := m.Called(ctx, boardID, filePath, mimeType, sizeBytes)
    return args.Get(0).(models.BoardAsset), args.Error(1)
}
func (m *mockWhiteboardRepo) GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error) {
    args := m.Called(ctx, assetID)
    return args.Get(0).(models.BoardAsset), args.Error(1)
}

func TestWhiteboardService_GetOrCreateBoard_CreatesFirstPage(t *testing.T) {
    repo := new(mockWhiteboardRepo)
    svc := service.NewWhiteboardService(repo)

    board := models.Board{ID: "board-1", CourseID: "course-1", TutorID: "tutor-1"}
    repo.On("GetOrCreateBoard", mock.Anything, "course-1", "tutor-1").Return(board, nil)
    // Нет страниц → сервис создаёт первую
    repo.On("GetPagesByBoard", mock.Anything, "board-1").Return([]models.BoardPage{}, nil)
    firstPage := models.BoardPage{ID: "page-1", BoardID: "board-1", Title: "Страница 1", Position: 0}
    repo.On("CreatePage", mock.Anything, "board-1", "Страница 1", 0).Return(firstPage, nil)

    result, err := svc.GetOrCreateBoard(context.Background(), "course-1", "tutor-1")
    assert.NoError(t, err)
    assert.Equal(t, "board-1", result.ID)
    assert.Len(t, result.Pages, 1)
    assert.Equal(t, "Страница 1", result.Pages[0].Title)
    repo.AssertExpectations(t)
}

func TestWhiteboardService_GetOrCreateBoard_ExistingPages(t *testing.T) {
    repo := new(mockWhiteboardRepo)
    svc := service.NewWhiteboardService(repo)

    board := models.Board{ID: "board-1", CourseID: "course-1", TutorID: "tutor-1"}
    pages := []models.BoardPage{{ID: "page-1", BoardID: "board-1", Title: "Урок 1"}}
    repo.On("GetOrCreateBoard", mock.Anything, "course-1", "tutor-1").Return(board, nil)
    repo.On("GetPagesByBoard", mock.Anything, "board-1").Return(pages, nil)

    result, err := svc.GetOrCreateBoard(context.Background(), "course-1", "tutor-1")
    assert.NoError(t, err)
    assert.Len(t, result.Pages, 1)
    // CreatePage не вызывается если страницы уже есть
    repo.AssertNotCalled(t, "CreatePage")
}
```

- [ ] **Step 2: Запустить тесты — убедиться что падают**

```bash
go test ./service/ -run TestWhiteboardService -v
```

Ожидается: FAIL (сервис не создан)

- [ ] **Step 3: Написать сервис**

```go
package service

import (
    "context"
    "encoding/json"
    "tutorgo/models"
    "tutorgo/repository"
)

type WhiteboardService interface {
    GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.BoardWithPages, error)
    ValidateInvite(ctx context.Context, inviteID string) (models.BoardWithPages, error)
    CreatePage(ctx context.Context, boardID, tutorID, title string) (models.BoardPage, error)
    UpdatePage(ctx context.Context, pageID, boardID string, req models.UpdateBoardPageRequest) (models.BoardPage, error)
    DeletePage(ctx context.Context, pageID, boardID string) error
    SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error
    CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error)
    DeleteInvite(ctx context.Context, boardID string) error
    SaveAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error)
    GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error)
}

type whiteboardService struct {
    repo repository.WhiteboardRepository
}

func NewWhiteboardService(repo repository.WhiteboardRepository) WhiteboardService {
    return &whiteboardService{repo: repo}
}

func (s *whiteboardService) GetOrCreateBoard(ctx context.Context, courseID, tutorID string) (models.BoardWithPages, error) {
    board, err := s.repo.GetOrCreateBoard(ctx, courseID, tutorID)
    if err != nil {
        return models.BoardWithPages{}, err
    }
    pages, err := s.repo.GetPagesByBoard(ctx, board.ID)
    if err != nil {
        return models.BoardWithPages{}, err
    }
    if len(pages) == 0 {
        first, err := s.repo.CreatePage(ctx, board.ID, "Страница 1", 0)
        if err != nil {
            return models.BoardWithPages{}, err
        }
        pages = []models.BoardPage{first}
    }
    return models.BoardWithPages{Board: board, Pages: pages}, nil
}

func (s *whiteboardService) ValidateInvite(ctx context.Context, inviteID string) (models.BoardWithPages, error) {
    board, err := s.repo.GetBoardByInvite(ctx, inviteID)
    if err != nil {
        return models.BoardWithPages{}, ErrNotFound
    }
    pages, err := s.repo.GetPagesByBoard(ctx, board.ID)
    if err != nil {
        return models.BoardWithPages{}, err
    }
    return models.BoardWithPages{Board: board, Pages: pages}, nil
}

func (s *whiteboardService) CreatePage(ctx context.Context, boardID, tutorID, title string) (models.BoardPage, error) {
    pages, err := s.repo.GetPagesByBoard(ctx, boardID)
    if err != nil {
        return models.BoardPage{}, err
    }
    return s.repo.CreatePage(ctx, boardID, title, len(pages))
}

func (s *whiteboardService) UpdatePage(ctx context.Context, pageID, boardID string, req models.UpdateBoardPageRequest) (models.BoardPage, error) {
    page, err := s.repo.GetPageByID(ctx, pageID)
    if err != nil || page.BoardID != boardID {
        return models.BoardPage{}, ErrNotFound
    }
    return s.repo.UpdatePage(ctx, pageID, req)
}

func (s *whiteboardService) DeletePage(ctx context.Context, pageID, boardID string) error {
    return s.repo.DeletePage(ctx, pageID, boardID)
}

func (s *whiteboardService) SaveSnapshot(ctx context.Context, pageID string, snapshot json.RawMessage) error {
    return s.repo.SaveSnapshot(ctx, pageID, snapshot)
}

func (s *whiteboardService) CreateInvite(ctx context.Context, boardID string) (models.BoardInvite, error) {
    return s.repo.CreateInvite(ctx, boardID)
}

func (s *whiteboardService) DeleteInvite(ctx context.Context, boardID string) error {
    return s.repo.DeleteInvite(ctx, boardID)
}

func (s *whiteboardService) SaveAsset(ctx context.Context, boardID, filePath, mimeType string, sizeBytes int) (models.BoardAsset, error) {
    return s.repo.CreateAsset(ctx, boardID, filePath, mimeType, sizeBytes)
}

func (s *whiteboardService) GetAsset(ctx context.Context, assetID string) (models.BoardAsset, error) {
    return s.repo.GetAsset(ctx, assetID)
}
```

- [ ] **Step 4: Запустить тесты — убедиться что проходят**

```bash
go test ./service/ -run TestWhiteboardService -v
```

Ожидается: PASS

- [ ] **Step 5: Коммит**

```bash
git add service/whiteboard.go service/whiteboard_test.go
git commit -m "feat: add whiteboard service with tests"
```

---

## Task 5: REST Handlers

**Files:**
- Create: `handlers/whiteboard.go`

- [ ] **Step 1: Создать хендлеры**

```go
package handlers

import (
    "fmt"
    "io"
    "log/slog"
    "net/http"
    "os"
    "path/filepath"

    "tutorgo/models"
    "tutorgo/service"

    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
)

type WhiteboardHandler struct {
    svc    service.WhiteboardService
    log    *slog.Logger
    wsHub  *WbHubManager
}

func NewWhiteboardHandler(svc service.WhiteboardService, log *slog.Logger, wsHub *WbHubManager) *WhiteboardHandler {
    return &WhiteboardHandler{svc: svc, log: log, wsHub: wsHub}
}

func (h *WhiteboardHandler) GetBoardByCourse(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    result, err := h.svc.GetOrCreateBoard(c.Request.Context(), c.Param("courseId"), tutorID)
    if err != nil {
        h.log.Error("GetOrCreateBoard", slog.String("error", err.Error()))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusOK, result)
}

func (h *WhiteboardHandler) CreatePage(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    var req models.CreateBoardPageRequest
    if !bindAndValidate(c, &req) {
        return
    }
    page, err := h.svc.CreatePage(c.Request.Context(), c.Param("boardId"), tutorID, req.Title)
    if err != nil {
        handleServiceError(c, err)
        return
    }
    c.JSON(http.StatusCreated, page)
}

func (h *WhiteboardHandler) UpdatePage(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    _ = tutorID
    var req models.UpdateBoardPageRequest
    if !bindAndValidate(c, &req) {
        return
    }
    // boardId приходит как query param или из другого источника;
    // для простоты передаём пустую строку — repo делает DELETE WHERE id=$1 AND board_id=$2
    // Реальная проверка принадлежности: в сервисе GetPageByID сверяет board_id
    page, err := h.svc.UpdatePage(c.Request.Context(), c.Param("pageId"), c.Query("boardId"), req)
    if err != nil {
        handleServiceError(c, err)
        return
    }
    c.JSON(http.StatusOK, page)
}

func (h *WhiteboardHandler) DeletePage(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    _ = tutorID
    if err := h.svc.DeletePage(c.Request.Context(), c.Param("pageId"), c.Query("boardId")); err != nil {
        handleServiceError(c, err)
        return
    }
    c.JSON(http.StatusNoContent, nil)
}

func (h *WhiteboardHandler) CreateInvite(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    inv, err := h.svc.CreateInvite(c.Request.Context(), c.Param("boardId"))
    if err != nil {
        h.log.Error("CreateInvite", slog.String("error", err.Error()))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }
    c.JSON(http.StatusCreated, inv)
}

func (h *WhiteboardHandler) DeleteInvite(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    if err := h.svc.DeleteInvite(c.Request.Context(), c.Param("boardId")); err != nil {
        handleServiceError(c, err)
        return
    }
    c.JSON(http.StatusNoContent, nil)
}

func (h *WhiteboardHandler) JoinByInvite(c *gin.Context) {
    result, err := h.svc.ValidateInvite(c.Request.Context(), c.Param("token"))
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "invite not found"})
        return
    }
    c.JSON(http.StatusOK, result)
}

func (h *WhiteboardHandler) UploadAsset(c *gin.Context) {
    tutorID := c.GetString("tutorID")
    if tutorID == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
        return
    }
    file, header, err := c.Request.FormFile("file")
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
        return
    }
    defer file.Close()

    const maxSize = 20 << 20 // 20MB
    if header.Size > maxSize {
        c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large (max 20MB)"})
        return
    }

    dir := "uploads/board-assets"
    if err := os.MkdirAll(dir, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
        return
    }

    ext := filepath.Ext(header.Filename)
    filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
    dst := filepath.Join(dir, filename)

    out, err := os.Create(dst)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
        return
    }
    defer out.Close()

    if _, err := io.Copy(out, file); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
        return
    }

    mimeType := header.Header.Get("Content-Type")
    asset, err := h.svc.SaveAsset(c.Request.Context(), c.Param("boardId"), dst, mimeType, int(header.Size))
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
        return
    }

    c.JSON(http.StatusCreated, models.BoardAssetResponse{
        ID:  asset.ID,
        URL: fmt.Sprintf("/public/board-assets/%s", asset.ID),
    })
}

func (h *WhiteboardHandler) ServeAsset(c *gin.Context) {
    asset, err := h.svc.GetAsset(c.Request.Context(), c.Param("id"))
    if err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
        return
    }
    c.File(asset.FilePath)
}
```

- [ ] **Step 2: Проверить компиляцию**

```bash
go build ./...
```

- [ ] **Step 3: Коммит**

```bash
git add handlers/whiteboard.go
git commit -m "feat: add whiteboard REST handlers"
```

---

## Task 6: WebSocket Hub

**Files:**
- Create: `handlers/whiteboard_ws.go`

- [ ] **Step 1: Создать Hub**

```go
package handlers

import (
    "context"
    "encoding/json"
    "log/slog"
    "net/http"
    "sync"
    "time"

    "tutorgo/service"

    "github.com/bep/debounce"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
    CheckOrigin:     func(r *http.Request) bool { return true },
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

// WbMsg — сообщение между клиентом и хабом
type WbMsg struct {
    Type    string          `json:"type"`
    Payload json.RawMessage `json:"payload,omitempty"`
    X       float64         `json:"x,omitempty"`
    Y       float64         `json:"y,omitempty"`
    PeerID  string          `json:"peerId,omitempty"`
}

// wbClient — одно WebSocket-соединение
type wbClient struct {
    hub    *wbHub
    conn   *websocket.Conn
    send   chan []byte
    peerID string
}

// wbHub — координатор для одной страницы доски
type wbHub struct {
    pageID       string
    clients      map[*wbClient]bool
    broadcast    chan wbBroadcast
    register     chan *wbClient
    unregister   chan *wbClient
    snapshot     json.RawMessage
    saveDebounce func(f func())
    svc          service.WhiteboardService
    log          *slog.Logger
}

type wbBroadcast struct {
    sender *wbClient
    data   []byte
}

func newWbHub(pageID string, snapshot json.RawMessage, svc service.WhiteboardService, log *slog.Logger) *wbHub {
    h := &wbHub{
        pageID:     pageID,
        clients:    make(map[*wbClient]bool),
        broadcast:  make(chan wbBroadcast, 64),
        register:   make(chan *wbClient),
        unregister: make(chan *wbClient),
        snapshot:   snapshot,
        svc:        svc,
        log:        log,
    }
    h.saveDebounce = debounce.New(2 * time.Second)
    return h
}

func (h *wbHub) run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
            // Отправить текущий snapshot новому клиенту
            if h.snapshot != nil {
                msg, _ := json.Marshal(WbMsg{Type: "snapshot", Payload: h.snapshot})
                client.send <- msg
            }

        case client := <-h.unregister:
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
                // Немедленно сохранить snapshot при отключении
                if h.snapshot != nil {
                    snap := h.snapshot
                    go func() {
                        if err := h.svc.SaveSnapshot(context.Background(), h.pageID, snap); err != nil {
                            h.log.Error("save snapshot on disconnect", slog.String("error", err.Error()))
                        }
                    }()
                }
            }

        case msg := <-h.broadcast:
            var parsed WbMsg
            if err := json.Unmarshal(msg.data, &parsed); err != nil {
                continue
            }

            if parsed.Type == "update" {
                h.snapshot = parsed.Payload
                // Дебаунс-сохранение в БД
                h.saveDebounce(func() {
                    snap := h.snapshot
                    if err := h.svc.SaveSnapshot(context.Background(), h.pageID, snap); err != nil {
                        h.log.Error("save snapshot", slog.String("error", err.Error()))
                    }
                })
            }

            // Добавить peerId отправителя в cursor-сообщения
            if parsed.Type == "cursor" {
                parsed.PeerID = msg.sender.peerID
                msg.data, _ = json.Marshal(parsed)
            }

            // Разослать всем кроме отправителя
            for client := range h.clients {
                if client == msg.sender {
                    continue
                }
                select {
                case client.send <- msg.data:
                default:
                    delete(h.clients, client)
                    close(client.send)
                }
            }
        }
    }
}

func (c *wbClient) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()
    c.conn.SetReadLimit(512 * 1024)
    c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })
    for {
        _, data, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
        c.hub.broadcast <- wbBroadcast{sender: c, data: data}
    }
}

func (c *wbClient) writePump() {
    ticker := time.NewTicker(30 * time.Second)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()
    for {
        select {
        case msg, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
                return
            }
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}

// WbHubManager — хранит активные хабы (по одному на pageID)
type WbHubManager struct {
    mu   sync.Mutex
    hubs map[string]*wbHub
    svc  service.WhiteboardService
    log  *slog.Logger
}

func NewWbHubManager(svc service.WhiteboardService, log *slog.Logger) *WbHubManager {
    return &WbHubManager{
        hubs: make(map[string]*wbHub),
        svc:  svc,
        log:  log,
    }
}

func (m *WbHubManager) getOrCreate(pageID string, snapshot json.RawMessage) *wbHub {
    m.mu.Lock()
    defer m.mu.Unlock()
    if hub, ok := m.hubs[pageID]; ok {
        return hub
    }
    hub := newWbHub(pageID, snapshot, m.svc, m.log)
    m.hubs[pageID] = hub
    go hub.run()
    return hub
}

// ServeWS — хендлер GET /ws/board/:pageId?token=...
func (m *WbHubManager) ServeWS(svc service.WhiteboardService) gin.HandlerFunc {
    return func(c *gin.Context) {
        pageID := c.Param("pageId")
        token := c.Query("token")

        // Авторизация: JWT или invite UUID
        var authorized bool
        if c.GetString("tutorID") != "" {
            authorized = true
        } else if token != "" {
            // Проверяем invite token
            if _, err := svc.ValidateInvite(c.Request.Context(), token); err == nil {
                authorized = true
            }
        }
        if !authorized {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
            return
        }

        // Загружаем актуальный snapshot страницы из БД
        page, err := svc.GetAsset(c.Request.Context(), pageID)
        _ = page
        _ = err
        // Для snapshot используем сервис через репозиторий напрямую —
        // добавим GetPageSnapshot в сервис (см. ниже через WhiteboardHandler.getPageSnapshot)
        var snapshot json.RawMessage

        conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
        if err != nil {
            return
        }

        hub := m.getOrCreate(pageID, snapshot)
        client := &wbClient{
            hub:    hub,
            conn:   conn,
            send:   make(chan []byte, 256),
            peerID: uuid.New().String(),
        }
        hub.register <- client

        go client.writePump()
        client.readPump()
    }
}
```

> **Примечание:** ServeWS требует доступа к snapshot страницы при подключении. Добавь метод `GetPageSnapshot(ctx, pageID) (json.RawMessage, error)` в `WhiteboardService` и `WhiteboardRepository`, который возвращает `board_pages.snapshot`. Затем замени заглушку `var snapshot json.RawMessage` на:
>
> ```go
> snapshot, _ = svc.GetPageSnapshot(c.Request.Context(), pageID)
> ```

- [ ] **Step 2: Добавить GetPageSnapshot в интерфейсы**

В `repository/whiteboard.go` добавить в интерфейс:
```go
GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error)
```

Реализация:
```go
func (r *whiteboardRepository) GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error) {
    var snap json.RawMessage
    err := r.conn.QueryRow(ctx,
        `SELECT snapshot FROM board_pages WHERE id = $1`, pageID,
    ).Scan(&snap)
    return snap, err
}
```

В `service/whiteboard.go` добавить в интерфейс и реализацию:
```go
GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error)
```
```go
func (s *whiteboardService) GetPageSnapshot(ctx context.Context, pageID string) (json.RawMessage, error) {
    return s.repo.GetPageSnapshot(ctx, pageID)
}
```

Заменить заглушку в `ServeWS`:
```go
snapshot, _ = svc.GetPageSnapshot(c.Request.Context(), pageID)
```

- [ ] **Step 3: Проверить компиляцию**

```bash
go build ./...
```

- [ ] **Step 4: Коммит**

```bash
git add handlers/whiteboard_ws.go repository/whiteboard.go service/whiteboard.go
git commit -m "feat: add whiteboard WebSocket hub"
```

---

## Task 7: Router Wiring

**Files:**
- Modify: `router/router.go`

- [ ] **Step 1: Добавить whiteboard в Router**

В начало `router/router.go` добавить в блок Repositories:
```go
whiteboardRepo := repository.NewWhiteboardRepository(pool)
```

В блок Services:
```go
whiteboardService := service.NewWhiteboardService(whiteboardRepo)
```

В блок Handlers:
```go
wbHubManager := handlers.NewWbHubManager(whiteboardService, log)
whiteboardHandler := handlers.NewWhiteboardHandler(whiteboardService, log, wbHubManager)
```

В блок маршрутов (внутри `protected := r.Group("", middleware.Auth(cfg.JWTSecret))`):
```go
// Whiteboard
r.GET("/public/board/join/:token", whiteboardHandler.JoinByInvite)
r.GET("/public/board-assets/:id", whiteboardHandler.ServeAsset)
r.GET("/ws/board/:pageId", wbHubManager.ServeWS(whiteboardService))

protected.GET("/boards/course/:courseId", whiteboardHandler.GetBoardByCourse)
protected.POST("/boards/:boardId/pages", whiteboardHandler.CreatePage)
protected.PUT("/board-pages/:pageId", whiteboardHandler.UpdatePage)
protected.DELETE("/board-pages/:pageId", whiteboardHandler.DeletePage)
protected.POST("/boards/:boardId/invite", whiteboardHandler.CreateInvite)
protected.DELETE("/boards/:boardId/invite", whiteboardHandler.DeleteInvite)
protected.POST("/boards/:boardId/assets", whiteboardHandler.UploadAsset)
```

Также увеличить лимит тела запроса для `/boards/:boardId/assets` (upload 20MB):
```go
r.Use(func(c *gin.Context) {
    if c.FullPath() == "/boards/:boardId/assets" {
        c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20<<20)
    } else {
        c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
    }
    c.Next()
})
```

- [ ] **Step 2: Перевести gorilla/websocket из indirect в direct**

```bash
cd /home/dragonbrn/tutorgo && go get github.com/gorilla/websocket
go get github.com/bep/debounce
```

- [ ] **Step 3: Проверить компиляцию и тесты**

```bash
go build ./...
go test ./...
```

- [ ] **Step 4: Коммит**

```bash
git add router/router.go go.mod go.sum
git commit -m "feat: wire whiteboard routes and WebSocket hub into router"
```

---

## Task 8: Frontend Types + API Client

**Files:**
- Modify: `frontend/src/types/api.ts`
- Create: `frontend/src/lib/api/whiteboard.ts`

- [ ] **Step 1: Добавить типы в api.ts**

```typescript
// Добавить в конец frontend/src/types/api.ts

export interface Board {
  id: string
  course_id: string
  tutor_id: string
  created_at: string
}

export interface BoardPage {
  id: string
  board_id: string
  title: string
  snapshot: unknown | null
  position: number
  created_at: string
  updated_at: string
}

export interface BoardInvite {
  id: string
  board_id: string
  created_at: string
}

export interface BoardAsset {
  id: string
  board_id: string
  file_path: string
  mime_type: string
  size_bytes: number
  created_at: string
}

export interface BoardAssetResponse {
  id: string
  url: string
}

export interface BoardWithPages extends Board {
  pages: BoardPage[]
}
```

- [ ] **Step 2: Создать API-клиент**

```typescript
// frontend/src/lib/api/whiteboard.ts
import { api } from './client'
import type { BoardWithPages, BoardPage, BoardInvite, BoardAssetResponse } from '@/types/api'

export const whiteboardApi = {
  getBoardByCourse: (courseId: string) =>
    api.get<BoardWithPages>(`/boards/course/${courseId}`).then((r) => r.data),

  createPage: (boardId: string, title: string) =>
    api.post<BoardPage>(`/boards/${boardId}/pages`, { title }).then((r) => r.data),

  updatePage: (pageId: string, boardId: string, data: { title?: string; position?: number }) =>
    api.put<BoardPage>(`/board-pages/${pageId}?boardId=${boardId}`, data).then((r) => r.data),

  deletePage: (pageId: string, boardId: string) =>
    api.delete(`/board-pages/${pageId}?boardId=${boardId}`),

  createInvite: (boardId: string) =>
    api.post<BoardInvite>(`/boards/${boardId}/invite`).then((r) => r.data),

  deleteInvite: (boardId: string) =>
    api.delete(`/boards/${boardId}/invite`),

  joinByInvite: (token: string) =>
    api.get<BoardWithPages>(`/public/board/join/${token}`).then((r) => r.data),

  uploadAsset: (boardId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post<BoardAssetResponse>(`/boards/${boardId}/assets`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    }).then((r) => r.data)
  },
}

export function getWsUrl(pageId: string, token?: string): string {
  const base = (process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8080')
    .replace(/^http/, 'ws')
  const params = token ? `?token=${token}` : ''
  return `${base}/ws/board/${pageId}${params}`
}
```

- [ ] **Step 3: Коммит**

```bash
cd frontend && git add src/types/api.ts src/lib/api/whiteboard.ts
git commit -m "feat: add whiteboard frontend types and API client"
```

---

## Task 9: React Query Hooks

**Files:**
- Create: `frontend/src/lib/hooks/useWhiteboard.ts`

- [ ] **Step 1: Создать хуки**

```typescript
// frontend/src/lib/hooks/useWhiteboard.ts
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { whiteboardApi } from '@/lib/api/whiteboard'

export const boardKeys = {
  byCourse: (courseId: string) => ['board', 'course', courseId] as const,
}

export function useBoardByCourse(courseId: string) {
  return useQuery({
    queryKey: boardKeys.byCourse(courseId),
    queryFn:  () => whiteboardApi.getBoardByCourse(courseId),
    enabled:  !!courseId,
  })
}

export function useCreatePage(boardId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (title: string) => whiteboardApi.createPage(boardId, title),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board'] }),
  })
}

export function useUpdatePage(boardId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ pageId, data }: { pageId: string; data: { title?: string; position?: number } }) =>
      whiteboardApi.updatePage(pageId, boardId, data),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board'] }),
  })
}

export function useDeletePage(boardId: string) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (pageId: string) => whiteboardApi.deletePage(pageId, boardId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['board'] }),
  })
}

export function useCreateInvite(boardId: string) {
  return useMutation({
    mutationFn: () => whiteboardApi.createInvite(boardId),
  })
}

export function useDeleteInvite(boardId: string) {
  return useMutation({
    mutationFn: () => whiteboardApi.deleteInvite(boardId),
  })
}

export function useJoinByInvite(token: string) {
  return useQuery({
    queryKey: ['board', 'invite', token],
    queryFn:  () => whiteboardApi.joinByInvite(token),
    enabled:  !!token,
  })
}
```

- [ ] **Step 2: Коммит**

```bash
git add src/lib/hooks/useWhiteboard.ts
git commit -m "feat: add whiteboard React Query hooks"
```

---

## Task 10: useWhiteboardSync Hook

**Files:**
- Create: `frontend/src/components/whiteboard/useWhiteboardSync.ts`

- [ ] **Step 1: Установить tldraw**

```bash
cd /home/dragonbrn/tutorgo/frontend && npm install @tldraw/tldraw
```

- [ ] **Step 2: Создать хук**

```typescript
// frontend/src/components/whiteboard/useWhiteboardSync.ts
'use client'

import { useEffect, useRef, useState, useCallback } from 'react'
import {
  createTLStore,
  defaultShapeUtils,
  loadSnapshot,
  TLStoreWithStatus,
  TLRecord,
} from '@tldraw/tldraw'
import { getWsUrl } from '@/lib/api/whiteboard'
import type { BoardPage } from '@/types/api'

type ConnStatus = 'connecting' | 'connected' | 'disconnected'

interface CursorInfo {
  peerId: string
  x: number
  y: number
}

interface SyncResult {
  store: ReturnType<typeof createTLStore>
  status: ConnStatus
  cursors: CursorInfo[]
  sendCursor: (x: number, y: number) => void
}

export function useWhiteboardSync(page: BoardPage | null, token?: string): SyncResult {
  const [status, setStatus] = useState<ConnStatus>('connecting')
  const [cursors, setCursors] = useState<CursorInfo[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const retryRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const [store] = useState(() => createTLStore({ shapeUtils: [...defaultShapeUtils] }))

  const connect = useCallback(() => {
    if (!page) return
    const ws = new WebSocket(getWsUrl(page.id, token))
    wsRef.current = ws

    ws.onopen = () => setStatus('connected')

    ws.onmessage = (e) => {
      const msg = JSON.parse(e.data) as {
        type: string
        payload?: { added?: Record<string, TLRecord>; updated?: Record<string, [TLRecord, TLRecord]>; removed?: Record<string, TLRecord> }
        peerId?: string
        x?: number
        y?: number
      }

      if (msg.type === 'snapshot' && msg.payload) {
        store.mergeRemoteChanges(() => {
          const { added, updated, removed } = msg.payload!
          if (added)   store.put(Object.values(added))
          if (updated) store.put(Object.values(updated).map(([, next]) => next))
          if (removed) store.remove(Object.keys(removed) as any[])
        })
      }

      if (msg.type === 'update' && msg.payload) {
        store.mergeRemoteChanges(() => {
          const { added, updated, removed } = msg.payload!
          if (added)   store.put(Object.values(added))
          if (updated) store.put(Object.values(updated).map(([, next]) => next))
          if (removed) store.remove(Object.keys(removed) as any[])
        })
      }

      if (msg.type === 'cursor' && msg.peerId) {
        setCursors((prev) => {
          const others = prev.filter((c) => c.peerId !== msg.peerId)
          return [...others, { peerId: msg.peerId!, x: msg.x ?? 0, y: msg.y ?? 0 }]
        })
      }
    }

    ws.onclose = () => {
      setStatus('disconnected')
      retryRef.current = setTimeout(connect, 2000)
    }

    ws.onerror = () => ws.close()
  }, [page, token, store])

  useEffect(() => {
    connect()
    return () => {
      retryRef.current && clearTimeout(retryRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  // Слушаем изменения пользователя и отправляем по WS
  useEffect(() => {
    const unsub = store.listen(
      ({ changes }) => {
        if (wsRef.current?.readyState !== WebSocket.OPEN) return
        wsRef.current.send(JSON.stringify({ type: 'update', payload: changes }))
      },
      { source: 'user', scope: 'document' }
    )
    return unsub
  }, [store])

  const sendCursor = useCallback((x: number, y: number) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'cursor', x, y }))
    }
  }, [])

  return { store, status, cursors, sendCursor }
}
```

- [ ] **Step 3: Коммит**

```bash
git add src/components/whiteboard/useWhiteboardSync.ts
git commit -m "feat: add useWhiteboardSync hook (tldraw ↔ WebSocket)"
```

---

## Task 11: TldrawCanvas Component

**Files:**
- Create: `frontend/src/components/whiteboard/TldrawCanvas.tsx`

- [ ] **Step 1: Создать компонент**

```tsx
// frontend/src/components/whiteboard/TldrawCanvas.tsx
'use client'

import { Tldraw } from '@tldraw/tldraw'
import '@tldraw/tldraw/tldraw.css'
import { useWhiteboardSync } from './useWhiteboardSync'
import type { BoardPage } from '@/types/api'

interface Props {
  page: BoardPage | null
  token?: string
  boardId: string
}

export function TldrawCanvas({ page, token, boardId }: Props) {
  const { store, status, sendCursor } = useWhiteboardSync(page, token)

  return (
    <div className="relative w-full h-full">
      {status === 'disconnected' && (
        <div className="absolute top-2 left-1/2 -translate-x-1/2 z-10 bg-yellow-100 border border-yellow-300 text-yellow-800 text-sm px-3 py-1 rounded-full">
          ⚠ Переподключение...
        </div>
      )}
      <Tldraw
        store={store}
        onPointerMove={(info) => sendCursor(info.point.x, info.point.y)}
        inferDarkMode
      />
    </div>
  )
}
```

- [ ] **Step 2: Добавить обработку wb:insert-image**

`BoardToolbar` диспатчит `wb:insert-image` при загрузке файла. Добавь в `TldrawCanvas` обработку через `onMount`:

```tsx
import { Editor } from '@tldraw/tldraw'

// Внутри TldrawCanvas добавить ref и onMount:
const editorRef = useRef<Editor | null>(null)

useEffect(() => {
  const handler = (e: Event) => {
    const { url, width = 800, height = 600 } = (e as CustomEvent).detail
    const editor = editorRef.current
    if (!editor) return
    const assetId = `asset:${crypto.randomUUID()}` as any
    editor.createAssets([{ id: assetId, type: 'image', typeName: 'asset',
      props: { src: url, w: width, h: height, mimeType: 'image/png', name: 'image', isAnimated: false },
      meta: {} }])
    editor.createShape({ type: 'image', props: { assetId, w: width, h: height } })
  }
  window.addEventListener('wb:insert-image', handler)
  return () => window.removeEventListener('wb:insert-image', handler)
}, [])

// В JSX:
// <Tldraw store={store} onMount={(e) => { editorRef.current = e }} ... />
```

- [ ] **Step 3: Коммит**

```bash
git add src/components/whiteboard/TldrawCanvas.tsx
git commit -m "feat: add TldrawCanvas component"
```

---

## Task 12: PageSidebar + BoardToolbar

**Files:**
- Create: `frontend/src/components/whiteboard/PageSidebar.tsx`
- Create: `frontend/src/components/whiteboard/BoardToolbar.tsx`

- [ ] **Step 1: Создать PageSidebar**

```tsx
// frontend/src/components/whiteboard/PageSidebar.tsx
'use client'

import { useState } from 'react'
import type { BoardPage } from '@/types/api'
import { useCreatePage, useDeletePage } from '@/lib/hooks/useWhiteboard'

interface Props {
  boardId: string
  pages: BoardPage[]
  activePageId: string
  onSelect: (pageId: string) => void
}

export function PageSidebar({ boardId, pages, activePageId, onSelect }: Props) {
  const [newTitle, setNewTitle] = useState('')
  const [adding, setAdding] = useState(false)
  const createPage = useCreatePage(boardId)
  const deletePage = useDeletePage(boardId)

  const handleAdd = async () => {
    if (!newTitle.trim()) return
    await createPage.mutateAsync(newTitle.trim())
    setNewTitle('')
    setAdding(false)
  }

  return (
    <div className="w-48 flex-shrink-0 border-r border-gray-200 bg-gray-50 flex flex-col h-full">
      <div className="p-3 font-medium text-sm text-gray-600 border-b">Страницы</div>
      <div className="flex-1 overflow-y-auto">
        {pages.map((p) => (
          <div
            key={p.id}
            onClick={() => onSelect(p.id)}
            className={`group flex items-center justify-between px-3 py-2 text-sm cursor-pointer hover:bg-gray-100 ${
              p.id === activePageId ? 'bg-blue-50 text-blue-700 font-medium' : ''
            }`}
          >
            <span className="truncate">{p.title}</span>
            {pages.length > 1 && (
              <button
                onClick={(e) => { e.stopPropagation(); deletePage.mutate(p.id) }}
                className="opacity-0 group-hover:opacity-100 text-gray-400 hover:text-red-500 ml-1"
              >
                ×
              </button>
            )}
          </div>
        ))}
      </div>
      <div className="p-2 border-t">
        {adding ? (
          <div className="flex gap-1">
            <input
              autoFocus
              className="flex-1 border rounded px-2 py-1 text-xs"
              value={newTitle}
              onChange={(e) => setNewTitle(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') handleAdd(); if (e.key === 'Escape') setAdding(false) }}
              placeholder="Название..."
            />
            <button onClick={handleAdd} className="text-xs bg-blue-500 text-white px-2 rounded">+</button>
          </div>
        ) : (
          <button
            onClick={() => { setAdding(true); setNewTitle('') }}
            className="w-full text-xs text-gray-500 hover:text-gray-700 py-1"
          >
            + Страница
          </button>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Создать BoardToolbar**

```tsx
// frontend/src/components/whiteboard/BoardToolbar.tsx
'use client'

import { useState, useRef } from 'react'
import * as pdfjs from 'pdfjs-dist'
import { useCreateInvite } from '@/lib/hooks/useWhiteboard'
import { whiteboardApi } from '@/lib/api/whiteboard'

// Указать путь к воркеру pdfjs
pdfjs.GlobalWorkerOptions.workerSrc = `//cdnjs.cloudflare.com/ajax/libs/pdf.js/${pdfjs.version}/pdf.worker.min.js`

interface Props {
  boardId: string
  isGuest?: boolean
}

export function BoardToolbar({ boardId, isGuest = false }: Props) {
  const [copying, setCopying] = useState(false)
  const [uploading, setUploading] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)
  const createInvite = useCreateInvite(boardId)

  const handleCopyInvite = async () => {
    const inv = await createInvite.mutateAsync()
    const url = `${window.location.origin}/board/join/${inv.id}`
    await navigator.clipboard.writeText(url)
    setCopying(true)
    setTimeout(() => setCopying(false), 2000)
  }

  const handlePdfUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    try {
      if (file.type === 'application/pdf') {
        const arrayBuffer = await file.arrayBuffer()
        const pdf = await pdfjs.getDocument({ data: arrayBuffer }).promise
        for (let i = 1; i <= pdf.numPages; i++) {
          const page = await pdf.getPage(i)
          const viewport = page.getViewport({ scale: 1.5 })
          const canvas = document.createElement('canvas')
          canvas.width = viewport.width
          canvas.height = viewport.height
          const ctx = canvas.getContext('2d')!
          await page.render({ canvasContext: ctx, viewport }).promise
          const blob = await new Promise<Blob>((res) => canvas.toBlob((b) => res(b!), 'image/png'))
          const pngFile = new File([blob], `page-${i}.png`, { type: 'image/png' })
          const result = await whiteboardApi.uploadAsset(boardId, pngFile)
          // tldraw: вставить как image через editor API (делается в TldrawCanvas через ref)
          window.dispatchEvent(new CustomEvent('wb:insert-image', { detail: { url: `/api${result.url}`, width: viewport.width, height: viewport.height } }))
        }
      } else {
        const result = await whiteboardApi.uploadAsset(boardId, file)
        window.dispatchEvent(new CustomEvent('wb:insert-image', { detail: { url: `/api${result.url}` } }))
      }
    } finally {
      setUploading(false)
      if (fileRef.current) fileRef.current.value = ''
    }
  }

  return (
    <div className="flex items-center gap-2 px-3 py-2 border-b bg-white">
      {!isGuest && (
        <button
          onClick={handleCopyInvite}
          className="text-sm px-3 py-1 rounded bg-blue-50 hover:bg-blue-100 text-blue-700 border border-blue-200"
        >
          {copying ? '✓ Скопировано!' : '🔗 Пригласить ученика'}
        </button>
      )}
      <button
        onClick={() => fileRef.current?.click()}
        disabled={uploading}
        className="text-sm px-3 py-1 rounded bg-gray-50 hover:bg-gray-100 text-gray-700 border border-gray-200"
      >
        {uploading ? 'Загрузка...' : '📎 PDF / Изображение'}
      </button>
      <input
        ref={fileRef}
        type="file"
        accept="image/*,application/pdf"
        className="hidden"
        onChange={handlePdfUpload}
      />
    </div>
  )
}
```

- [ ] **Step 3: Установить pdfjs-dist**

```bash
cd /home/dragonbrn/tutorgo/frontend && npm install pdfjs-dist
```

- [ ] **Step 4: Коммит**

```bash
git add src/components/whiteboard/
git commit -m "feat: add PageSidebar and BoardToolbar components"
```

---

## Task 13: Tutor Board Page

**Files:**
- Create: `frontend/src/app/(dashboard)/boards/[courseId]/page.tsx`

- [ ] **Step 1: Создать страницу**

```tsx
// frontend/src/app/(dashboard)/boards/[courseId]/page.tsx
'use client'

import { useState } from 'react'
import { useBoardByCourse } from '@/lib/hooks/useWhiteboard'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
import { PageSidebar } from '@/components/whiteboard/PageSidebar'
import { BoardToolbar } from '@/components/whiteboard/BoardToolbar'

interface Props {
  params: { courseId: string }
}

export default function BoardPage({ params }: Props) {
  const { data: board, isLoading, error } = useBoardByCourse(params.courseId)
  const [activePageId, setActivePageId] = useState<string | null>(null)

  if (isLoading) return <div className="flex items-center justify-center h-screen text-gray-500">Загрузка доски...</div>
  if (error || !board) return <div className="flex items-center justify-center h-screen text-red-500">Ошибка загрузки доски</div>

  const currentPageId = activePageId ?? board.pages[0]?.id ?? null
  const currentPage = board.pages.find((p) => p.id === currentPageId) ?? null

  return (
    <div className="flex flex-col h-screen">
      <BoardToolbar boardId={board.id} />
      <div className="flex flex-1 overflow-hidden">
        <PageSidebar
          boardId={board.id}
          pages={board.pages}
          activePageId={currentPageId ?? ''}
          onSelect={setActivePageId}
        />
        <div className="flex-1">
          <TldrawCanvas page={currentPage} boardId={board.id} />
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Коммит**

```bash
git add src/app/\(dashboard\)/boards/
git commit -m "feat: add tutor board page"
```

---

## Task 14: Guest Join Page

**Files:**
- Create: `frontend/src/app/board/join/[token]/page.tsx`

> Этот маршрут находится вне `(dashboard)` — создаётся в корне `app/`.

- [ ] **Step 1: Создать папку и страницу**

```bash
mkdir -p /home/dragonbrn/tutorgo/frontend/src/app/board/join/\[token\]
```

```tsx
// frontend/src/app/board/join/[token]/page.tsx
'use client'

import { useState } from 'react'
import { useJoinByInvite } from '@/lib/hooks/useWhiteboard'
import { TldrawCanvas } from '@/components/whiteboard/TldrawCanvas'
import { PageSidebar } from '@/components/whiteboard/PageSidebar'
import { BoardToolbar } from '@/components/whiteboard/BoardToolbar'

interface Props {
  params: { token: string }
}

export default function GuestBoardPage({ params }: Props) {
  const { data: board, isLoading, error } = useJoinByInvite(params.token)
  const [activePageId, setActivePageId] = useState<string | null>(null)

  if (isLoading) return <div className="flex items-center justify-center h-screen text-gray-500">Загрузка доски...</div>
  if (error || !board) return (
    <div className="flex items-center justify-center h-screen">
      <div className="text-center">
        <p className="text-red-500 text-lg font-medium">Ссылка недействительна</p>
        <p className="text-gray-500 text-sm mt-1">Попросите репетитора прислать новую ссылку</p>
      </div>
    </div>
  )

  const currentPageId = activePageId ?? board.pages[0]?.id ?? null
  const currentPage = board.pages.find((p) => p.id === currentPageId) ?? null

  return (
    <div className="flex flex-col h-screen">
      <BoardToolbar boardId={board.id} isGuest />
      <div className="flex flex-1 overflow-hidden">
        <PageSidebar
          boardId={board.id}
          pages={board.pages}
          activePageId={currentPageId ?? ''}
          onSelect={setActivePageId}
        />
        <div className="flex-1">
          <TldrawCanvas page={currentPage} boardId={board.id} token={params.token} />
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Коммит**

```bash
git add src/app/board/
git commit -m "feat: add guest join board page"
```

---

## Task 15: Navigation Link from Course

**Files:**
- Modify: `frontend/src/app/(dashboard)/courses/[id]/page.tsx` (или аналогичный файл карточки курса)

- [ ] **Step 1: Найти файл деталей курса**

```bash
find /home/dragonbrn/tutorgo/frontend/src/app -name "page.tsx" | xargs grep -l "course\|Course" | head -5
```

- [ ] **Step 2: Добавить кнопку "Открыть доску"**

Найди блок с кнопками действий курса (рядом с "Редактировать" / "Архивировать") и добавь:

```tsx
import Link from 'next/link'

// В блоке кнопок курса:
<Link
  href={`/boards/${course.id}`}
  className="text-sm px-3 py-1.5 rounded border border-gray-200 hover:bg-gray-50 text-gray-700"
>
  🖊 Доска
</Link>
```

- [ ] **Step 3: Коммит**

```bash
git add src/app/\(dashboard\)/courses/
git commit -m "feat: add board link to course detail page"
```

---

## Checklist после завершения

- [ ] `go test ./...` проходит
- [ ] Фронтенд собирается без ошибок: `npm run build`
- [ ] Открыть доску курса → видна пустая страница
- [ ] Нарисовать что-то → обновить страницу → рисунок сохранился
- [ ] Открыть в двух вкладках → изменения в одной видны в другой
- [ ] Кнопка "Пригласить ученика" копирует рабочую ссылку
- [ ] Открыть ссылку → доска доступна без логина
- [ ] Загрузить PDF → страницы появляются на холсте

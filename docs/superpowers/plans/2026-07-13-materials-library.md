# Materials Library + Synced Media Player — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Препод хранит файлы в личной библиотеке и запускает аудио/видео с доски так, что play/pause/seek применяются у обоих участников одновременно.

**Architecture:** Одна таблица `materials` (adjacency list: папка = строка без `file_path`), пять REST-эндпоинтов по образцу существующего `WhiteboardHandler.UploadAsset`. Синхронизация плеера — через **существующий** WS доски: хаб уже ретранслирует неизвестные типы сообщений вербатим (`handlers/whiteboard_ws.go:160`, ветка `default:`), поэтому серверный код WS **не меняется вообще**. Плеер — нативные `<audio controls>` / `<video controls>`; локальные события `onPlay/onPause/onSeeked` уходят в WS, входящие применяются к элементу под флагом-глушилкой (иначе эхо-петля).

**Tech Stack:** Go 1.x + Gin + pgx + goose; Next.js 15 + React + axios + Excalidraw 0.18.1; тесты — testify/mock (Go) и `node:test` (frontend).

**Spec:** `docs/superpowers/specs/2026-07-13-materials-library-design.md`

## Global Constraints

- Лимит файла — 50 МБ. Существующий, не меняем (`router.go` multipart limit; фронт отсекает до аплоада константой `MAX_ASSET_BYTES` в `ExcalidrawCanvas.tsx:38`).
- Все REST-роуты библиотеки — под `middleware.Auth`, скоуп по `tutorID := c.GetString("tutorID")` с nil-guard (401, если пусто).
- Ключи в S3: `materials/<uuid><ext>`.
- TTL presigned-ссылки для материалов — **4 часа** (ссылку получает ученик по WS и переполучить не может).
- Библиотеку (панель) видит только препод: гейт `!isGuest`. Плеер видят оба.
- Ошибки сервиса — только `service.ErrNotFound / ErrForbidden / ErrConflict / ErrBadRequest`, хендлер отдаёт их через существующий `handleServiceError(c, err)`.
- Русский язык в UI-текстах и комментариях — как в остальном коде доски.
- Никаких новых зависимостей.

## Parallelization

| Агент | Задачи | Зависимости |
|---|---|---|
| **A (backend)** | Task 1, Task 2 | нет |
| **B (media sync)** | Task 3 | нет (контракт WS-сообщения зафиксирован ниже) |
| **C (панель)** | Task 4 | нет (контракт API зафиксирован ниже) |
| **интеграция** | Task 5 | нужны 3 и 4 (и 2 для ручного прогона) |

Задачи 1–2 последовательны внутри агента A. Задачи 3 и 4 не пересекаются по файлам.

## File Structure

**Создать (backend):**
- `migrations/025_materials.sql` — таблица.
- `models/material.go` — `Material`, `CreateFolderRequest`, `MaterialResponse`.
- `repository/material.go` — `MaterialRepository` (интерфейс + pgx-реализация).
- `service/material.go` — `MaterialService` (валидация владения и правил).
- `service/material_test.go` — testify/mock на репозиторий.
- `handlers/material.go` — `MaterialHandler` (5 методов).

**Изменить (backend):**
- `router/router.go` — проводка репо/сервиса/хендлера + 5 роутов.

**Создать (frontend):**
- `frontend/src/components/whiteboard/mediaSync.ts` — чистые типы и логика применения (тестируемая без DOM).
- `frontend/src/components/whiteboard/mediaSync.test.ts` — `node:test`.
- `frontend/src/components/whiteboard/useMediaPlayer.ts` — хук: состояние плеера + приём/отправка.
- `frontend/src/components/whiteboard/MediaPlayer.tsx` — вьюха (нативные controls).
- `frontend/src/components/whiteboard/MaterialsPanel.tsx` — сайд-панель библиотеки.
- `frontend/src/lib/api/materials.ts` — API-клиент.

**Изменить (frontend):**
- `frontend/src/types/api.ts` — типы `Material`, `MaterialKind`.
- `frontend/src/components/whiteboard/useExcalidrawSync.ts` — `sendMedia` + приём `media`.
- `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx` — кнопка, панель, плеер, роутинг по типу файла.

---

## Контракты (общие для всех агентов)

### REST

```
GET    /materials?parent_id=<uuid|пусто>   → 200 []MaterialResponse
POST   /materials/folder  {name, parent_id?}  → 201 MaterialResponse
POST   /materials         multipart: file, parent_id?  → 201 MaterialResponse
DELETE /materials/:id     → 204
GET    /materials/:id/url → 302 Location: <presigned>
```

`MaterialResponse` (JSON):

```json
{ "id": "uuid", "kind": "folder|file", "name": "unit3.mp3",
  "mime_type": "audio/mpeg", "size_bytes": 4211234,
  "created_at": "2026-07-13T10:00:00Z" }
```

У папок `mime_type` = `""`, `size_bytes` = 0. Сортировка списка: папки первыми, внутри группы по `name`.

### WS-сообщение плеера

Тип один — `media`. Отдельного `media_req` нет: запрос состояния — это `action: "req"`.

```ts
type MediaAction = 'open' | 'play' | 'pause' | 'seek' | 'close' | 'req'

interface MediaPayload {
  action: MediaAction
  url?: string        // presigned; только в 'open'
  mimeType?: string   // только в 'open'
  name?: string       // только в 'open'
  position?: number   // секунды; в 'open' | 'play' | 'pause' | 'seek'
}
```

Кадр в сокете: `{"type":"media","payload":{...}}`. Сервер ретранслирует всем, кроме отправителя (ветка `default:` в `wbHub.run()`).

---

## Task 1: Схема, модель, репозиторий (агент A)

**Files:**
- Create: `migrations/025_materials.sql`
- Create: `models/material.go`
- Create: `repository/material.go`

**Interfaces:**
- Consumes: ничего.
- Produces: `models.Material`, `models.CreateFolderRequest`, `models.MaterialResponse`, `repository.MaterialRepository` с методами:
  `ListByParent(ctx, tutorID string, parentID *string) ([]models.Material, error)`,
  `GetByID(ctx, id string) (models.Material, error)`,
  `CreateFolder(ctx, tutorID, name string, parentID *string) (models.Material, error)`,
  `CreateFile(ctx, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error)`,
  `HasChildren(ctx, id string) (bool, error)`,
  `Delete(ctx, id, tutorID string) error`.

- [ ] **Step 1: Миграция**

Создать `migrations/025_materials.sql`:

```sql
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
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (kind = 'folder' OR file_path IS NOT NULL)
);
CREATE INDEX idx_materials_tutor_parent ON materials (tutor_id, parent_id);

-- +goose Down
DROP TABLE materials;
```

- [ ] **Step 2: Применить миграцию**

Run: `make migrate-up`
Expected: `OK   025_materials.sql`

- [ ] **Step 3: Модель**

Создать `models/material.go`:

```go
package models

import "time"

type Material struct {
	ID        string    `json:"id"`
	TutorID   string    `json:"tutor_id"`
	ParentID  *string   `json:"parent_id"`
	Kind      string    `json:"kind"` // "folder" | "file"
	Name      string    `json:"name"`
	FilePath  string    `json:"-"` // ключ в S3, наружу не отдаём
	MimeType  string    `json:"mime_type"`
	SizeBytes int       `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateFolderRequest struct {
	Name     string  `json:"name"      validate:"required,min=1,max=100"`
	ParentID *string `json:"parent_id" validate:"omitempty,uuid"`
}

type MaterialResponse struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int       `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

func NewMaterialResponse(m Material) MaterialResponse {
	return MaterialResponse{
		ID:        m.ID,
		Kind:      m.Kind,
		Name:      m.Name,
		MimeType:  m.MimeType,
		SizeBytes: m.SizeBytes,
		CreatedAt: m.CreatedAt,
	}
}
```

- [ ] **Step 4: Репозиторий**

Создать `repository/material.go`:

```go
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

// scanCols — общий список колонок и приёмник, чтобы SELECT'ы не разъезжались.
const materialCols = `id, tutor_id, parent_id, kind, name,
	COALESCE(file_path, ''), COALESCE(mime_type, ''), COALESCE(size_bytes, 0), created_at`

func (r *materialRepository) ListByParent(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error) {
	// parent_id IS NOT DISTINCT FROM $2 — единый запрос и для корня (NULL), и
	// для папки: обычное `=` с NULL всегда даёт NULL (пустой список).
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
```

- [ ] **Step 5: Сборка**

Run: `go build ./...`
Expected: без вывода (успех).

- [ ] **Step 6: Commit**

```bash
git add migrations/025_materials.sql models/material.go repository/material.go
git commit -m "feat(materials): schema, model, repository"
```

---

## Task 2: Сервис, хендлеры, роуты (агент A)

**Files:**
- Create: `service/material.go`
- Create: `service/material_test.go`
- Create: `handlers/material.go`
- Modify: `router/router.go`

**Interfaces:**
- Consumes: всё из Task 1.
- Produces: `service.MaterialService`:
  `List(ctx, tutorID string, parentID *string) ([]models.Material, error)`,
  `CreateFolder(ctx, tutorID string, req models.CreateFolderRequest) (models.Material, error)`,
  `CreateFile(ctx, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error)`,
  `Delete(ctx, id, tutorID string) (models.Material, error)` — возвращает удалённый материал, чтобы хендлер снёс объект в S3,
  `GetFile(ctx, id, tutorID string) (models.Material, error)`.
  Хендлер: `handlers.NewMaterialHandler(svc service.MaterialService, store *storage.Client, log *slog.Logger) *MaterialHandler` с методами `List`, `CreateFolder`, `Upload`, `Delete`, `GetURL`.

- [ ] **Step 1: Написать падающий тест сервиса**

Создать `service/material_test.go`. Мок пишем прямо в файле — так же, как в `service/whiteboard_test.go`:

```go
package service_test

import (
	"context"
	"errors"
	"testing"

	"tutorgo/models"
	"tutorgo/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockMaterialRepo struct{ mock.Mock }

func (m *mockMaterialRepo) ListByParent(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error) {
	args := m.Called(ctx, tutorID, parentID)
	return args.Get(0).([]models.Material), args.Error(1)
}
func (m *mockMaterialRepo) GetByID(ctx context.Context, id string) (models.Material, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.Material), args.Error(1)
}
func (m *mockMaterialRepo) CreateFolder(ctx context.Context, tutorID, name string, parentID *string) (models.Material, error) {
	args := m.Called(ctx, tutorID, name, parentID)
	return args.Get(0).(models.Material), args.Error(1)
}
func (m *mockMaterialRepo) CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error) {
	args := m.Called(ctx, tutorID, name, filePath, mimeType, sizeBytes, parentID)
	return args.Get(0).(models.Material), args.Error(1)
}
func (m *mockMaterialRepo) HasChildren(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}
func (m *mockMaterialRepo) Delete(ctx context.Context, id, tutorID string) error {
	return m.Called(ctx, id, tutorID).Error(0)
}

func strPtr(s string) *string { return &s }

// Папка другого препода как parent — отказ, чужое дерево недоступно.
func TestCreateFolder_ForeignParent(t *testing.T) {
	repo := new(mockMaterialRepo)
	repo.On("GetByID", mock.Anything, "p1").
		Return(models.Material{ID: "p1", TutorID: "other", Kind: "folder"}, nil)

	svc := service.NewMaterialService(repo)
	_, err := svc.CreateFolder(context.Background(), "me",
		models.CreateFolderRequest{Name: "Аудирование", ParentID: strPtr("p1")})

	assert.ErrorIs(t, err, service.ErrNotFound)
	repo.AssertNotCalled(t, "CreateFolder", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// Родителем может быть только папка, но не файл.
func TestCreateFolder_ParentIsFile(t *testing.T) {
	repo := new(mockMaterialRepo)
	repo.On("GetByID", mock.Anything, "f1").
		Return(models.Material{ID: "f1", TutorID: "me", Kind: "file"}, nil)

	svc := service.NewMaterialService(repo)
	_, err := svc.CreateFolder(context.Background(), "me",
		models.CreateFolderRequest{Name: "Аудирование", ParentID: strPtr("f1")})

	assert.ErrorIs(t, err, service.ErrBadRequest)
}

// Непустую папку удалять нельзя — тот же контракт, что у курса с уроками.
func TestDelete_NonEmptyFolder(t *testing.T) {
	repo := new(mockMaterialRepo)
	repo.On("GetByID", mock.Anything, "d1").
		Return(models.Material{ID: "d1", TutorID: "me", Kind: "folder"}, nil)
	repo.On("HasChildren", mock.Anything, "d1").Return(true, nil)

	svc := service.NewMaterialService(repo)
	_, err := svc.Delete(context.Background(), "d1", "me")

	assert.ErrorIs(t, err, service.ErrConflict)
	repo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything, mock.Anything)
}

// Удаление файла возвращает материал: хендлеру нужен file_path, чтобы снести объект в S3.
func TestDelete_FileReturnsMaterial(t *testing.T) {
	repo := new(mockMaterialRepo)
	m := models.Material{ID: "f1", TutorID: "me", Kind: "file", FilePath: "materials/f1.mp3"}
	repo.On("GetByID", mock.Anything, "f1").Return(m, nil)
	repo.On("Delete", mock.Anything, "f1", "me").Return(nil)

	svc := service.NewMaterialService(repo)
	got, err := repoDelete(svc)

	assert.NoError(t, err)
	assert.Equal(t, "materials/f1.mp3", got.FilePath)
	_ = errors.New // держим импорт errors задействованным
}

func repoDelete(svc service.MaterialService) (models.Material, error) {
	return svc.Delete(context.Background(), "f1", "me")
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./service/ -run TestCreateFolder_ForeignParent`
Expected: FAIL — `undefined: service.NewMaterialService`

- [ ] **Step 3: Сервис**

Создать `service/material.go`:

```go
package service

import (
	"context"
	"errors"

	"tutorgo/models"
	"tutorgo/repository"

	"github.com/jackc/pgx/v5"
)

type MaterialService interface {
	List(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error)
	CreateFolder(ctx context.Context, tutorID string, req models.CreateFolderRequest) (models.Material, error)
	CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error)
	// Delete возвращает удалённый материал: хендлеру нужен FilePath, чтобы
	// снести объект в S3 после того, как строка исчезла из БД.
	Delete(ctx context.Context, id, tutorID string) (models.Material, error)
	GetFile(ctx context.Context, id, tutorID string) (models.Material, error)
}

type materialService struct {
	repo repository.MaterialRepository
}

func NewMaterialService(repo repository.MaterialRepository) MaterialService {
	return &materialService{repo: repo}
}

// requireOwnFolder проверяет, что parentID (если задан) — существующая папка
// этого препода. Возвращает ErrNotFound для чужого/несуществующего и
// ErrBadRequest, если это файл, а не папка.
func (s *materialService) requireOwnFolder(ctx context.Context, tutorID string, parentID *string) error {
	if parentID == nil {
		return nil
	}
	parent, err := s.repo.GetByID(ctx, *parentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if parent.TutorID != tutorID {
		return ErrNotFound
	}
	if parent.Kind != "folder" {
		return ErrBadRequest
	}
	return nil
}

func (s *materialService) List(ctx context.Context, tutorID string, parentID *string) ([]models.Material, error) {
	if err := s.requireOwnFolder(ctx, tutorID, parentID); err != nil {
		return nil, err
	}
	return s.repo.ListByParent(ctx, tutorID, parentID)
}

func (s *materialService) CreateFolder(ctx context.Context, tutorID string, req models.CreateFolderRequest) (models.Material, error) {
	if err := s.requireOwnFolder(ctx, tutorID, req.ParentID); err != nil {
		return models.Material{}, err
	}
	return s.repo.CreateFolder(ctx, tutorID, req.Name, req.ParentID)
}

func (s *materialService) CreateFile(ctx context.Context, tutorID, name, filePath, mimeType string, sizeBytes int, parentID *string) (models.Material, error) {
	if err := s.requireOwnFolder(ctx, tutorID, parentID); err != nil {
		return models.Material{}, err
	}
	return s.repo.CreateFile(ctx, tutorID, name, filePath, mimeType, sizeBytes, parentID)
}

func (s *materialService) Delete(ctx context.Context, id, tutorID string) (models.Material, error) {
	m, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Material{}, ErrNotFound
	}
	if err != nil {
		return models.Material{}, err
	}
	if m.TutorID != tutorID {
		return models.Material{}, ErrNotFound
	}
	if m.Kind == "folder" {
		// ponytail: рекурсивного удаления нет — оно требует обхода дерева и
		// пакетной чистки S3. Непустую папку просто не даём удалить, как курс с
		// уроками. Если начнёт мешать — рекурсия по parent_id + батч Remove.
		has, err := s.repo.HasChildren(ctx, id)
		if err != nil {
			return models.Material{}, err
		}
		if has {
			return models.Material{}, ErrConflict
		}
	}
	if err := s.repo.Delete(ctx, id, tutorID); err != nil {
		return models.Material{}, err
	}
	return m, nil
}

func (s *materialService) GetFile(ctx context.Context, id, tutorID string) (models.Material, error) {
	m, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Material{}, ErrNotFound
	}
	if err != nil {
		return models.Material{}, err
	}
	if m.TutorID != tutorID || m.Kind != "file" {
		return models.Material{}, ErrNotFound
	}
	return m, nil
}
```

- [ ] **Step 4: Тесты зелёные**

Run: `go test ./service/ -run TestCreateFolder -v && go test ./service/ -run TestDelete -v`
Expected: PASS по всем четырём тестам.

- [ ] **Step 5: Хендлеры**

Создать `handlers/material.go`:

```go
package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"time"

	"tutorgo/models"
	"tutorgo/service"
	"tutorgo/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MaterialHandler struct {
	svc   service.MaterialService
	store *storage.Client
	log   *slog.Logger
}

func NewMaterialHandler(svc service.MaterialService, store *storage.Client, log *slog.Logger) *MaterialHandler {
	return &MaterialHandler{svc: svc, store: store, log: log}
}

// optionalUUID возвращает nil для пустой строки — корень дерева.
func optionalUUID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// GET /materials?parent_id=
func (h *MaterialHandler) List(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	items, err := h.svc.List(c.Request.Context(), tutorID, optionalUUID(c.Query("parent_id")))
	if err != nil {
		handleServiceError(c, err)
		return
	}
	resp := make([]models.MaterialResponse, 0, len(items))
	for _, m := range items {
		resp = append(resp, models.NewMaterialResponse(m))
	}
	c.JSON(http.StatusOK, resp)
}

// POST /materials/folder
func (h *MaterialHandler) CreateFolder(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.CreateFolderRequest
	if !bindAndValidate(c, &req) {
		return
	}
	m, err := h.svc.CreateFolder(c.Request.Context(), tutorID, req)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.NewMaterialResponse(m))
}

// POST /materials — multipart: file, parent_id
func (h *MaterialHandler) Upload(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Лимит тела (50 МБ) уже стоит в глобальном middleware — как в UploadAsset.
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	defer file.Close()

	const maxSize = 50 << 20
	if header.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large (max 50MB)"})
		return
	}

	ext := filepath.Ext(header.Filename)
	key := fmt.Sprintf("materials/%s%s", uuid.New().String(), ext)
	mimeType := header.Header.Get("Content-Type")

	if err := h.store.Put(c.Request.Context(), key, file, header.Size, mimeType); err != nil {
		h.log.Error("upload material to storage", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	m, err := h.svc.CreateFile(c.Request.Context(), tutorID, header.Filename, key,
		mimeType, int(header.Size), optionalUUID(c.PostForm("parent_id")))
	if err != nil {
		// Строка не записалась — сносим только что залитый объект, иначе он
		// осиротеет в бакете (тот же откат, что в UploadAsset).
		_ = h.store.Remove(c.Request.Context(), key)
		handleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, models.NewMaterialResponse(m))
}

// DELETE /materials/:id
func (h *MaterialHandler) Delete(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	m, err := h.svc.Delete(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	if m.Kind == "file" && m.FilePath != "" {
		// Объект чистим после БД: осиротевший объект дешевле, чем строка,
		// указывающая в пустоту.
		if err := h.store.Remove(c.Request.Context(), m.FilePath); err != nil {
			h.log.Error("remove material object", "err", err, "key", m.FilePath)
		}
	}
	c.Status(http.StatusNoContent)
}

// GET /materials/:id/url — 302 на presigned-ссылку.
func (h *MaterialHandler) GetURL(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	m, err := h.svc.GetFile(c.Request.Context(), c.Param("id"), tutorID)
	if err != nil {
		handleServiceError(c, err)
		return
	}
	// 4 часа: ссылку получает ученик по WS и переполучить её не может, поэтому
	// она должна пережить весь урок (у board-assets хватает 20 минут — там
	// картинка перезапрашивается через редирект).
	url, err := h.store.PresignGet(c.Request.Context(), m.FilePath, 4*time.Hour)
	if err != nil {
		h.log.Error("presign material", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "presigned link error"})
		return
	}
	c.Redirect(http.StatusFound, url)
}
```

- [ ] **Step 6: Проводка в роутере**

В `router/router.go` рядом с существующими репозиториями (около строки 36) добавить:

```go
	materialRepo := repository.NewMaterialRepository(pool)
```

Рядом с сервисами (около строки 54):

```go
	materialService := service.NewMaterialService(materialRepo)
```

После создания `store` и `whiteboardHandler` (около строки 81):

```go
	materialHandler := handlers.NewMaterialHandler(materialService, store, log)
```

В группе `auth` (рядом с whiteboard-роутами, около строки 217):

```go
		// Materials library
		auth.GET("/materials", materialHandler.List)
		auth.POST("/materials/folder", materialHandler.CreateFolder)
		auth.POST("/materials", materialHandler.Upload)
		auth.DELETE("/materials/:id", materialHandler.Delete)
		auth.GET("/materials/:id/url", materialHandler.GetURL)
```

- [ ] **Step 7: Сборка и тесты**

Run: `go build ./... && go test ./...`
Expected: сборка без вывода, все тесты PASS.

- [ ] **Step 8: Commit**

```bash
git add service/material.go service/material_test.go handlers/material.go router/router.go
git commit -m "feat(materials): service, handlers, routes"
```

---

## Task 3: Синхронный плеер (агент B)

**Files:**
- Create: `frontend/src/components/whiteboard/mediaSync.ts`
- Create: `frontend/src/components/whiteboard/mediaSync.test.ts`
- Create: `frontend/src/components/whiteboard/useMediaPlayer.ts`
- Create: `frontend/src/components/whiteboard/MediaPlayer.tsx`
- Modify: `frontend/src/components/whiteboard/useExcalidrawSync.ts`

**Interfaces:**
- Consumes: ничего (WS-контракт зафиксирован в разделе «Контракты»).
- Produces:
  - `mediaSync.ts`: `type MediaAction`, `interface MediaPayload`, `interface MediaState { url: string; mimeType: string; name: string }`, `function nextMediaState(prev: MediaState | null, p: MediaPayload): MediaState | null`, `function isPlayable(mimeType: string): boolean`.
  - `useMediaPlayer.ts`: `function useMediaPlayer(sendMedia: (p: MediaPayload) => void): { media: MediaState | null; mediaRef: RefObject<HTMLMediaElement | null>; needsGesture: boolean; open(p: {url,mimeType,name}): void; close(): void; onLocalPlay(): void; onLocalPause(): void; onLocalSeeked(): void; receive(p: MediaPayload): void; resume(): void }`.
  - `MediaPlayer.tsx`: `function MediaPlayer(props: { player: ReturnType<typeof useMediaPlayer>; canClose: boolean }): JSX.Element | null`.
  - `useExcalidrawSync.ts`: добавляет в `ExcalidrawSyncResult` поле `sendMedia: (p: MediaPayload) => void` и принимает 4-й аргумент `onMedia?: (p: MediaPayload) => void`.

- [ ] **Step 1: Написать падающий тест чистой логики**

Создать `frontend/src/components/whiteboard/mediaSync.test.ts` (стиль — как `excalidrawSync.test.ts`: `node:test` + `node:assert/strict`):

```ts
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { nextMediaState, isPlayable } from './mediaSync.ts'

test('open вводит новое состояние', () => {
  const s = nextMediaState(null, {
    action: 'open',
    url: 'https://s3/u.mp3',
    mimeType: 'audio/mpeg',
    name: 'u.mp3',
  })
  assert.deepEqual(s, { url: 'https://s3/u.mp3', mimeType: 'audio/mpeg', name: 'u.mp3' })
})

test('close сбрасывает состояние', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  assert.equal(nextMediaState(open, { action: 'close' }), null)
})

test('play/pause/seek не меняют открытый файл', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  for (const action of ['play', 'pause', 'seek'] as const) {
    assert.deepEqual(nextMediaState(open, { action, position: 4 }), open)
  }
})

test('open без url игнорируется — не сносим играющий файл битым кадром', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  assert.deepEqual(nextMediaState(open, { action: 'open' }), open)
})

test('req не меняет состояние', () => {
  const open = { url: 'u', mimeType: 'audio/mpeg', name: 'n' }
  assert.deepEqual(nextMediaState(open, { action: 'req' }), open)
})

test('isPlayable: только аудио и видео', () => {
  assert.equal(isPlayable('audio/mpeg'), true)
  assert.equal(isPlayable('video/mp4'), true)
  assert.equal(isPlayable('application/pdf'), false)
  assert.equal(isPlayable(''), false)
})
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `cd frontend && node --test --experimental-strip-types src/components/whiteboard/mediaSync.test.ts`
Expected: FAIL — не может разрешить `./mediaSync.ts`.

(Если команда для тестов в проекте оформлена иначе — посмотреть, как запускается `excalidrawSync.test.ts`, и использовать тот же способ.)

- [ ] **Step 3: Чистая логика**

Создать `frontend/src/components/whiteboard/mediaSync.ts`:

```ts
export type MediaAction = 'open' | 'play' | 'pause' | 'seek' | 'close' | 'req'

export interface MediaPayload {
  action: MediaAction
  url?: string
  mimeType?: string
  name?: string
  /** Позиция в секундах. */
  position?: number
}

export interface MediaState {
  url: string
  mimeType: string
  name: string
}

/** Какой файл открыт после применения кадра. Транспортные действия
 *  (play/pause/seek/req) файл не меняют — они правят только сам элемент. */
export function nextMediaState(
  prev: MediaState | null,
  p: MediaPayload
): MediaState | null {
  if (p.action === 'close') return null
  if (p.action === 'open') {
    // Битый open (без url) игнорируем, иначе он снесёт играющий файл.
    if (!p.url || !p.mimeType) return prev
    return { url: p.url, mimeType: p.mimeType, name: p.name ?? '' }
  }
  return prev
}

export function isPlayable(mimeType: string): boolean {
  return mimeType.startsWith('audio/') || mimeType.startsWith('video/')
}
```

- [ ] **Step 4: Тесты зелёные**

Run: `cd frontend && node --test --experimental-strip-types src/components/whiteboard/mediaSync.test.ts`
Expected: PASS, 6 тестов.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/whiteboard/mediaSync.ts frontend/src/components/whiteboard/mediaSync.test.ts
git commit -m "feat(materials): media sync state logic"
```

- [ ] **Step 6: Транспорт в useExcalidrawSync**

В `frontend/src/components/whiteboard/useExcalidrawSync.ts`:

1. Импорт типа:

```ts
import type { MediaPayload } from './mediaSync'
```

2. В `ExcalidrawSyncResult` добавить поле:

```ts
  sendMedia: (p: MediaPayload) => void
```

3. Сигнатура хука получает 4-й аргумент:

```ts
export function useExcalidrawSync(
  page: BoardPage | null,
  token?: string,
  identity?: BoardIdentity,
  onMedia?: (p: MediaPayload) => void
): ExcalidrawSyncResult {
```

4. Сразу после `const myUid = identity?.uid` — ref на колбэк, чтобы `connect` не пересоздавался при каждом рендере родителя:

```ts
  // Колбэк живёт в ref: иначе новый инлайн-обработчик на каждом рендере попал бы
  // в deps connect и передёргивал бы WS-соединение.
  const onMediaRef = useRef(onMedia)
  onMediaRef.current = onMedia
```

5. В `ws.onmessage`, рядом с блоком `if (msg.type === 'file')`, добавить:

```ts
      if (msg.type === 'media') {
        onMediaRef.current?.(msg.payload as MediaPayload)
        return
      }
```

6. Рядом с `sendCursor` (около строки 407) добавить отправку:

```ts
  const sendMedia = useCallback((p: MediaPayload) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'media', payload: p }))
    }
  }, [])
```

7. Добавить `sendMedia` в возвращаемый объект хука.

- [ ] **Step 7: Хук плеера**

Создать `frontend/src/components/whiteboard/useMediaPlayer.ts`:

```ts
'use client'

import { useCallback, useRef, useState } from 'react'
import { nextMediaState, type MediaPayload, type MediaState } from './mediaSync'

/** Расхождение больше этого — подтягиваем позицию; меньше — не дёргаем элемент,
 *  чтобы не заикался звук. Сеть даёт 100–300 мс, для аудирования незаметно. */
const SEEK_TOLERANCE_SEC = 0.5

export function useMediaPlayer(sendMedia: (p: MediaPayload) => void) {
  const [media, setMedia] = useState<MediaState | null>(null)
  const [needsGesture, setNeedsGesture] = useState(false)
  const mediaRef = useRef<HTMLMediaElement | null>(null)
  // Пока применяем удалённый кадр, локальные onPlay/onPause/onSeeked молчат —
  // иначе приём порождает отправку и получается эхо-петля между вкладками.
  const applyingRef = useRef(false)
  const stateRef = useRef<MediaState | null>(null)
  stateRef.current = media

  const withSuppressed = useCallback((fn: () => void) => {
    applyingRef.current = true
    fn()
    // Снимаем флаг в макрозадаче: play/pause/seeked прилетают асинхронно.
    setTimeout(() => {
      applyingRef.current = false
    }, 0)
  }, [])

  /** Приём кадра из WS. */
  const receive = useCallback(
    (p: MediaPayload) => {
      // Спрашивают состояние: если у нас открыт файл — отвечаем своим кадром.
      if (p.action === 'req') {
        const cur = stateRef.current
        const el = mediaRef.current
        if (cur) {
          sendMedia({
            action: 'open',
            url: cur.url,
            mimeType: cur.mimeType,
            name: cur.name,
            position: el?.currentTime ?? 0,
          })
          if (el && !el.paused) {
            sendMedia({ action: 'play', position: el.currentTime })
          }
        }
        return
      }

      setMedia((prev) => nextMediaState(prev, p))

      const el = mediaRef.current
      if (!el) return

      withSuppressed(() => {
        if (p.position !== undefined) {
          if (Math.abs(el.currentTime - p.position) > SEEK_TOLERANCE_SEC) {
            el.currentTime = p.position
          }
        }
        if (p.action === 'play') {
          void el.play().catch(() => {
            // Автоплей заблокирован: браузер не даёт играть без жеста
            // пользователя. Показываем кнопку «Включить звук».
            setNeedsGesture(true)
          })
        }
        if (p.action === 'pause') el.pause()
        if (p.action === 'close') el.pause()
      })
    },
    [sendMedia, withSuppressed]
  )

  const open = useCallback(
    (p: { url: string; mimeType: string; name: string }) => {
      setMedia({ url: p.url, mimeType: p.mimeType, name: p.name })
      setNeedsGesture(false)
      sendMedia({ action: 'open', ...p, position: 0 })
    },
    [sendMedia]
  )

  const close = useCallback(() => {
    setMedia(null)
    sendMedia({ action: 'close' })
  }, [sendMedia])

  const onLocalPlay = useCallback(() => {
    if (applyingRef.current) return
    setNeedsGesture(false)
    sendMedia({ action: 'play', position: mediaRef.current?.currentTime ?? 0 })
  }, [sendMedia])

  const onLocalPause = useCallback(() => {
    if (applyingRef.current) return
    sendMedia({ action: 'pause', position: mediaRef.current?.currentTime ?? 0 })
  }, [sendMedia])

  const onLocalSeeked = useCallback(() => {
    if (applyingRef.current) return
    sendMedia({ action: 'seek', position: mediaRef.current?.currentTime ?? 0 })
  }, [sendMedia])

  /** Клик по «Включить звук»: жест есть, повторяем play. */
  const resume = useCallback(() => {
    setNeedsGesture(false)
    void mediaRef.current?.play()
  }, [])

  return {
    media,
    mediaRef,
    needsGesture,
    open,
    close,
    onLocalPlay,
    onLocalPause,
    onLocalSeeked,
    receive,
    resume,
  }
}
```

- [ ] **Step 8: Вьюха плеера**

Создать `frontend/src/components/whiteboard/MediaPlayer.tsx`. Никакого кастомного UI — нативные `controls` дают play/pause/seek бесплатно; визуал переделывается отдельно.

```tsx
'use client'

import type { useMediaPlayer } from './useMediaPlayer'

interface Props {
  player: ReturnType<typeof useMediaPlayer>
  /** Закрывать плеер может только препод. */
  canClose: boolean
}

export function MediaPlayer({ player, canClose }: Props) {
  const { media, mediaRef, needsGesture, close, onLocalPlay, onLocalPause, onLocalSeeked, resume } =
    player
  if (!media) return null

  const isVideo = media.mimeType.startsWith('video/')
  const common = {
    ref: mediaRef as React.RefObject<never>,
    src: media.url,
    controls: true,
    onPlay: onLocalPlay,
    onPause: onLocalPause,
    onSeeked: onLocalSeeked,
  }

  return (
    <div
      data-board-ui
      className="absolute bottom-4 left-1/2 -translate-x-1/2 z-20 flex flex-col gap-2 rounded-xl bg-white p-3 shadow-lg"
      style={{ width: isVideo ? 480 : 360 }}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm font-medium">{media.name}</span>
        {canClose && (
          <button onClick={close} className="text-sm text-gray-500 hover:text-gray-900">
            ✕
          </button>
        )}
      </div>

      {isVideo ? (
        <video {...common} className="w-full rounded-lg" />
      ) : (
        <audio {...common} className="w-full" />
      )}

      {needsGesture && (
        <button
          onClick={resume}
          className="rounded-lg bg-black px-3 py-1.5 text-sm font-medium text-white"
        >
          Включить звук
        </button>
      )}
    </div>
  )
}
```

- [ ] **Step 9: Типы сходятся**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок. (`ExcalidrawCanvas.tsx` пока не использует новые файлы — это Task 5.)

- [ ] **Step 10: Commit**

```bash
git add frontend/src/components/whiteboard/useMediaPlayer.ts \
        frontend/src/components/whiteboard/MediaPlayer.tsx \
        frontend/src/components/whiteboard/useExcalidrawSync.ts
git commit -m "feat(materials): synced media player over board WS"
```

---

## Task 4: API-клиент и панель библиотеки (агент C)

**Files:**
- Create: `frontend/src/lib/api/materials.ts`
- Create: `frontend/src/components/whiteboard/MaterialsPanel.tsx`
- Modify: `frontend/src/types/api.ts`

**Interfaces:**
- Consumes: REST-контракт из раздела «Контракты» (бэкенд пишется параллельно; на него не смотреть, писать по контракту).
- Produces:
  - `types/api.ts`: `type MaterialKind = 'folder' | 'file'`, `interface Material { id: string; kind: MaterialKind; name: string; mime_type: string; size_bytes: number; created_at: string }`.
  - `lib/api/materials.ts`: `materialsApi.list(parentId?: string): Promise<Material[]>`, `.createFolder(name: string, parentId?: string): Promise<Material>`, `.upload(file: File, parentId?: string): Promise<Material>`, `.remove(id: string): Promise<void>`, `.getUrl(id: string): Promise<string>`.
  - `MaterialsPanel.tsx`: `function MaterialsPanel(props: { onClose: () => void; onPick: (m: Material, url: string) => void }): JSX.Element`.

- [ ] **Step 1: Типы**

В `frontend/src/types/api.ts` добавить:

```ts
export type MaterialKind = 'folder' | 'file'

export interface Material {
  id: string
  kind: MaterialKind
  name: string
  mime_type: string
  size_bytes: number
  created_at: string
}
```

- [ ] **Step 2: API-клиент**

Создать `frontend/src/lib/api/materials.ts`:

```ts
import { api } from './client'
import type { Material } from '@/types/api'

export const materialsApi = {
  list: (parentId?: string) =>
    api
      .get<Material[]>('/materials', { params: parentId ? { parent_id: parentId } : {} })
      .then((r) => r.data),

  createFolder: (name: string, parentId?: string) =>
    api
      .post<Material>('/materials/folder', { name, parent_id: parentId ?? null })
      .then((r) => r.data),

  upload: (file: File, parentId?: string) => {
    const form = new FormData()
    form.append('file', file)
    if (parentId) form.append('parent_id', parentId)
    return api
      .post<Material>('/materials', form, {
        // false → axios снимает заголовок и браузер сам выставит
        // multipart/form-data с boundary (см. whiteboardApi.uploadAsset).
        headers: { 'Content-Type': false },
      })
      .then((r) => r.data)
  },

  remove: (id: string) => api.delete(`/materials/${id}`).then(() => undefined),

  /** Возвращает presigned-ссылку. Бэкенд отвечает 302 — берём итоговый URL
   *  после редиректа, поэтому запрос идёт как обычный GET. */
  getUrl: async (id: string): Promise<string> => {
    const r = await api.get(`/materials/${id}/url`, { responseType: 'blob' })
    return (r.request as XMLHttpRequest).responseURL
  },
}
```

**Важно про `getUrl`:** axios в браузере прозрачно следует за 302, и итоговая presigned-ссылка доступна в `request.responseURL`. Скачанное тело не используем — оно нужно только затем, чтобы редирект отработал. Если при ручной проверке `responseURL` окажется пустым, заменить эндпоинт на возврат JSON `{url}` вместо 302 (правка в `handlers/material.go:GetURL` — `c.JSON(200, gin.H{"url": url})`) и упростить клиент до `.then(r => r.data.url)`. Решение принимает интегратор в Task 5 по факту прогона.

- [ ] **Step 3: Панель**

Создать `frontend/src/components/whiteboard/MaterialsPanel.tsx`. Панель абсолютно позиционирована справа под кнопкой; данные — через React Query (в проекте он уже используется на страницах доски) либо через `useState` + `useEffect`, если query-клиента в этом дереве нет — проверить по соседним компонентам и сделать как принято.

Функциональность:
- Навигация по папкам: локальный `useState` пути `Array<{id: string; name: string}>`; корень — пустой массив. Хлебные крошки кликабельны.
- Список: папки (клик — вход внутрь), файлы (клик — `onPick`).
- Кнопка «Новая папка»: `prompt` для имени — визуал переделывается отдельно, диалог не пишем.
- Кнопка «Загрузить»: скрытый `<input type="file">`, отсечка по 50 МБ **до** аплоада (константа `MAX_ASSET_BYTES` = `50 * 1024 * 1024`), ошибки — через `toast.error` из `sonner`, как в остальном коде доски.
- Удаление: крестик у элемента, `confirm()`; на 409 показать «Папка не пуста».
- После create/upload/delete — перечитать текущую папку.

Клик по файлу: получить ссылку `const url = await materialsApi.getUrl(m.id)` и вызвать `onPick(m, url)`. Что делать дальше — решает родитель (Task 5), панель про плееры и доску ничего не знает.

Обёртка панели должна нести атрибут `data-board-ui` (как у существующего блока в `renderTopRightUI`) и `className="absolute right-2 top-14 z-20 w-72 rounded-xl bg-white p-3 shadow-lg"`.

- [ ] **Step 4: Типы сходятся**

Run: `cd frontend && npx tsc --noEmit`
Expected: без ошибок.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/api/materials.ts \
        frontend/src/components/whiteboard/MaterialsPanel.tsx \
        frontend/src/types/api.ts
git commit -m "feat(materials): api client and library panel"
```

---

## Task 5: Интеграция в доску (после 3 и 4)

**Files:**
- Modify: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx`

**Interfaces:**
- Consumes: `useMediaPlayer`, `MediaPlayer`, `isPlayable` (Task 3); `MaterialsPanel`, `materialsApi`, `Material` (Task 4); `sendMedia` из `useExcalidrawSync` (Task 3).
- Produces: ничего.

- [ ] **Step 1: Подключить плеер к WS**

В `ExcalidrawCanvas.tsx` заменить вызов хука синхронизации так, чтобы `onMedia` уходил в плеер. Порядок объявлений важен: `useMediaPlayer` нужен `sendMedia`, а `useExcalidrawSync` нужен `onMedia` — развязываем через ref:

```tsx
  // sendMedia появляется только после useExcalidrawSync, а тому нужен onMedia,
  // который живёт в плеере. Разрываем цикл ref'ом: плеер шлёт через актуальный
  // sendMedia, доска зовёт актуальный receive.
  const sendMediaRef = useRef<(p: MediaPayload) => void>(() => {})
  const player = useMediaPlayer(useCallback((p: MediaPayload) => sendMediaRef.current(p), []))
  const { status, onApiReady, onChange, sendCursor, broadcastViewport, registerFile, sendMedia } =
    useExcalidrawSync(page, token, identity, player.receive)
  sendMediaRef.current = sendMedia
```

- [ ] **Step 2: Спросить состояние при подключении**

Ученик, перезагрузивший вкладку посреди трека, должен догнать плеер. После установления соединения шлём `req`; тот, у кого файл открыт, ответит (логика ответа уже в `useMediaPlayer.receive`).

```tsx
  useEffect(() => {
    if (status !== 'connected') return
    sendMedia({ action: 'req' })
  }, [status, sendMedia])
```

- [ ] **Step 3: Состояние панели и кнопка**

Добавить `const [materialsOpen, setMaterialsOpen] = useState(false)`.

В `renderTopRightUI`, внутри существующего `<div data-board-ui>`, после кнопки картинки, добавить кнопку под гейтом `!isGuest`. Иконка — папка, тем же стилем, что у кнопки картинки (`display:flex; border:none; background:transparent; borderRadius:8; cursor:pointer; padding:'6px 8px'`), `title="Материалы"`, `onClick={() => setMaterialsOpen((v) => !v)}`:

```tsx
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none"
                     stroke="currentColor" strokeWidth="1.8"
                     strokeLinecap="round" strokeLinejoin="round">
                  <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
                </svg>
```

- [ ] **Step 4: Рендер панели и плеера**

Внутри корневого `<div className="relative w-full h-full...">`, рядом с `{pdfDialog && ...}`:

```tsx
        {!isGuest && materialsOpen && (
          <MaterialsPanel onClose={() => setMaterialsOpen(false)} onPick={handlePickMaterial} />
        )}
        <MediaPlayer player={player} canClose={!isGuest} />
```

- [ ] **Step 5: Роутинг по типу файла**

Добавить обработчик выбора файла. Медиа — в плеер (сообщение уходит обоим). Картинка и PDF — на доску существующими путями. Прочее — скачивание.

```tsx
  const handlePickMaterial = useCallback(
    async (m: Material, url: string) => {
      // Медиа: плеер у обоих участников.
      if (isPlayable(m.mime_type)) {
        player.open({ url, mimeType: m.mime_type, name: m.name })
        setMaterialsOpen(false)
        return
      }

      const api = apiRef.current
      if (!api) return
      // Точка вставки — центр текущего вьюпорта.
      const { scrollX, scrollY, zoom, width, height } = api.getAppState()
      const point = viewportCoordsToSceneCoords(
        { clientX: width / 2, clientY: height / 2 },
        { scrollX, scrollY, zoom }
      )

      try {
        if (m.mime_type.startsWith('image/')) {
          const blob = await fetch(url).then((r) => r.blob())
          const bmp = await createImageBitmap(blob)
          // ponytail: картинка из библиотеки перезаливается в S3 как board-asset —
          // повторное хранение одного файла дешевле, чем ветка «доска умеет
          // ссылаться на объекты вне board-assets». Схлопнуть, если начнёт мешать.
          await insertImageBlob(blob, m.mime_type, point, { w: bmp.width, h: bmp.height }, m.name)
          setMaterialsOpen(false)
          return
        }

        if (m.mime_type === 'application/pdf') {
          const blob = await fetch(url).then((r) => r.blob())
          const pdf = await loadPdf(new File([blob], m.name, { type: m.mime_type }))
          pdfRef.current = pdf
          setPdfDialog({ numPages: pdf.numPages, point })
          setMaterialsOpen(false)
          return
        }

        // Всё прочее просто отдаём файлом.
        window.open(url, '_blank')
      } catch {
        toast.error('Не удалось открыть материал')
      }
    },
    [player, insertImageBlob]
  )
```

Перед реализацией сверить сигнатуру `loadPdf` в `frontend/src/lib/pdf.ts` и существующий вызов в `onDropCapture` (`ExcalidrawCanvas.tsx:230-245`) — использовать ровно тот же способ загрузки PDF, что уже работает для drop.

- [ ] **Step 6: Проверка типов и сборка**

Run: `cd frontend && npx tsc --noEmit && npm run build`
Expected: без ошибок.

- [ ] **Step 7: Ручной прогон (обязателен — тут нет автотестов)**

Поднять бэкенд (`make run`) и фронт, открыть доску в двух окнах (препод + гость по инвайту):

1. Препод: кнопка «Материалы» → создать папку → зайти внутрь → загрузить mp3. Гость кнопки не видит.
2. Препод: клик по mp3 → плеер появился у обоих.
3. Play у препода → играет у обоих. Pause у ученика → встал у обоих. Перемотка у любого → позиция совпала.
4. Перезагрузить вкладку ученика во время игры → плеер вернулся с той же позицией (`req` → ответ).
5. Загрузить PDF → клик → открылся `PdfRangeDialog` → страницы легли на доску.
6. Удалить непустую папку → тост «Папка не пуста».

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/whiteboard/ExcalidrawCanvas.tsx
git commit -m "feat(materials): wire library panel and player into the board"
```

---

## Self-Review (выполнено при написании плана)

- **Покрытие спеки:** таблица и ключи S3 → Task 1; пять эндпоинтов и правила владения → Task 2; WS-контракт, эхо-глушилка, late-join, автоплей → Task 3; панель, папки, загрузка, удаление → Task 4; кнопка, гейт `!isGuest`, роутинг по типу файла → Task 5. Разделы спеки «что осталось за бортом» задач не порождают.
- **Расхождение со спекой (осознанное):** спека описывала отдельный тип `media_req`; в плане это `action: 'req'` внутри `media`. Один тип сообщения вместо двух — меньше кода на обеих сторонах, поведение то же.
- **Согласованность имён:** `sendMedia`, `receive`, `nextMediaState`, `isPlayable`, `materialsApi`, `MaterialResponse` употребляются одинаково во всех задачах.

# Серверный импорт PDF — план имплементации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** PDF конвертируется на сервере (River-джоба + pdftoppm), страницы доезжают до доски на 100%; клиентский рендер pdf.js удаляется.

**Architecture:** Один Docker-образ, две роли по env `ROLE` (`api` / `worker`). Очередь — River поверх существующего Postgres (enqueue транзакционен с записью импорта). События воркер→API — `pg_notify('board_events', …)`, API-слушатель пересылает их в WS-хабы досок существующим типом сообщения `file`. Клиент раскладывает pending-плейсхолдеры сразу (механизм уже есть), картинки проявляются по мере готовности.

**Tech Stack:** Go 1.x + Gin + pgx/v5, `github.com/riverqueue/river` (+ `riverdriver/riverpgxv5`), poppler-utils (`pdfinfo`, `pdftoppm`) шелл-аутом, Next.js/TypeScript фронт.

**Спека:** `docs/superpowers/specs/2026-07-17-pdf-server-import-design.md` — прочитать перед работой.

## Global Constraints

- Лимит файла: **50 МБ** (существующий `MAX_ASSET_BYTES` / глобальный multipart-лимит).
- Растр: **JPEG quality=85, 200 DPI**.
- Импорт доступен **только преподавателю** (tutor JWT, ownership-проверки на каждом роуте).
- Габариты страниц по проводу — **в пунктах PDF (pt)**; масштаб сцены (×1.5) применяет клиент.
- URL страниц по проводу — **относительные** (`/public/board-assets/{id}`); абсолютный префикс добавляет клиент.
- Ретраи джобы: **MaxAttempts = 5** (дефолтный backoff River).
- Все комментарии в коде — на русском, в стиле кодовой базы.
- Существующие тесты не должны ломаться: `make test` и `cd frontend && npx tsc --noEmit` зелёные после каждой задачи.

## Параллелизация (для subagent-driven)

- **Волна 1 (независимы):** Task 1, Task 2, Task 3, Task 6
- **Волна 2:** Task 4 (после 1), Task 5 (после 1, 2, 3)
- **Волна 3:** Task 7 (после 4), Task 8 (после 7; контракт API зафиксирован в этом плане, вёрстку можно начинать раньше на свой риск)

---

### Task 1: Миграция, модель и репозиторий импорта

**Files:**
- Create: `migrations/027_pdf_imports.sql`
- Create: `models/pdf_import.go`
- Create: `repository/pdf_import.go`

**Interfaces:**
- Consumes: `models.BoardAsset`, паттерн репозиториев (`repository/whiteboard.go`).
- Produces (нужно задачам 4, 5, 7):
  - `models.PdfImportPage{N int; AssetID string; W, H float64; Done bool}` (json: `n`, `asset_id`, `w`, `h`, `done`)
  - `models.PdfImport{ID, BoardID, PageID, TutorID, S3Key, Status string; Pages []PdfImportPage; Error *string}`
  - `models.PdfPageAssetKey(importID string, n int) string`
  - `repository.PdfImportRepository` (сигнатуры ниже)

- [ ] **Step 1: Миграция**

```sql
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
```

- [ ] **Step 2: Модель**

```go
// models/pdf_import.go
package models

import "fmt"

// PdfImportPage — одна страница импорта внутри jsonb-поля pages.
// До start AssetID пуст; воркер выставляет Done по мере рендера.
type PdfImportPage struct {
	N       int     `json:"n"`
	AssetID string  `json:"asset_id"`
	W       float64 `json:"w"` // пункты PDF (1/72"), масштаб сцены применяет клиент
	H       float64 `json:"h"`
	Done    bool    `json:"done"`
}

type PdfImport struct {
	ID      string
	BoardID string
	PageID  string
	TutorID string
	S3Key   string
	Status  string
	Pages   []PdfImportPage
	Error   *string
}

// PdfPageAssetKey — единственное место, где рождается S3-ключ страницы:
// его пишут в board_assets при start и по нему же льёт воркер.
func PdfPageAssetKey(importID string, n int) string {
	return fmt.Sprintf("board-assets/pdf/%s/%d.jpg", importID, n)
}

// PageSizePt — габариты страницы из pdfinfo, в пунктах.
type PageSizePt struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type PdfPreflightResponse struct {
	ImportID  string       `json:"import_id"`
	NumPages  int          `json:"num_pages"`
	PageSizes []PageSizePt `json:"page_sizes"`
}

type PdfImportPageOut struct {
	FileID string  `json:"file_id"`
	URL    string  `json:"url"` // относительный /public/board-assets/{id}
	W      float64 `json:"w"`   // пункты
	H      float64 `json:"h"`
}

type PdfStartResponse struct {
	Pages []PdfImportPageOut `json:"pages"`
}

type StartPdfImportRequest struct {
	From int `json:"from" validate:"required,min=1"`
	To   int `json:"to" validate:"required,min=1"`
}
```

- [ ] **Step 3: Репозиторий**

```go
// repository/pdf_import.go
package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"tutorgo/models"
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
		return nil, fmt.Errorf("import %s already started (status %s)", id, status)
	}
	var all []models.PdfImportPage
	if err := json.Unmarshal(pagesJSON, &all); err != nil {
		return nil, err
	}
	if from < 1 || to > len(all) || from > to {
		return nil, fmt.Errorf("range %d-%d out of 1-%d", from, to, len(all))
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
```

- [ ] **Step 4: Проверка сборки и миграции**

Run: `go build ./... && make migrate-up && make migrate-status`
Expected: сборка чистая, `027_pdf_imports.sql` в статусе applied.

- [ ] **Step 5: Commit**

```bash
git add migrations/027_pdf_imports.sql models/pdf_import.go repository/pdf_import.go
git commit -m "feat: таблица, модель и репозиторий PDF-импортов"
```

---

### Task 2: Пакет pdftool (pdfinfo + pdftoppm)

**Files:**
- Create: `pdftool/pdftool.go`
- Create: `pdftool/pdftool_test.go`
- Create: `pdftool/smoke_test.go`

**Interfaces:**
- Consumes: `models.PageSizePt` (Task 1; при параллельной работе продублируй тип локально не надо — Task 1 в той же волне, дождись его коммита либо согласуй через main).
- Produces (нужно задачам 4, 5):
  - `pdftool.ParseInfo(out string) ([]models.PageSizePt, error)` — чистый парсер
  - `pdftool.Info(ctx context.Context, path string) ([]models.PageSizePt, error)` — шелл-аут pdfinfo
  - `pdftool.RenderPage(ctx context.Context, pdfPath string, n, dpi int, outDir string) (string, error)` — рендер одной страницы, возвращает путь к JPEG

- [ ] **Step 1: Написать падающий тест парсера**

```go
// pdftool/pdftool_test.go
package pdftool

import "testing"

// Реальный формат вывода pdfinfo -f 1 -l N: строки "Page N size: W x H pts (…)".
const sampleInfo = `Title:          Учебник
Producer:       LibreOffice
Pages:          3
Page    1 size: 612 x 792 pts (letter)
Page    1 rot:  0
Page    2 size: 595.28 x 841.89 pts (A4)
Page    2 rot:  0
Page    3 size: 841.89 x 595.28 pts (A4)
Page    3 rot:  90
File size:      12345 bytes`

func TestParseInfo(t *testing.T) {
	sizes, err := ParseInfo(sampleInfo)
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 3 {
		t.Fatalf("want 3 pages, got %d", len(sizes))
	}
	if sizes[0].W != 612 || sizes[0].H != 792 {
		t.Errorf("page 1: got %+v", sizes[0])
	}
	if sizes[1].W != 595.28 {
		t.Errorf("page 2 W: got %v", sizes[1].W)
	}
	// альбомная страница — ширина больше высоты, порядок не путаем
	if sizes[2].W != 841.89 || sizes[2].H != 595.28 {
		t.Errorf("page 3: got %+v", sizes[2])
	}
}

func TestParseInfoNoPages(t *testing.T) {
	if _, err := ParseInfo("Producer: x\n"); err == nil {
		t.Fatal("want error for output without pages")
	}
}
```

- [ ] **Step 2: Прогнать — убедиться, что падает**

Run: `go test ./pdftool/`
Expected: FAIL (ParseInfo не определён).

- [ ] **Step 3: Реализация**

```go
// Package pdftool — шелл-ауты в poppler-utils (pdfinfo, pdftoppm).
// Шелл-аут, а не Go-библиотека: poppler переживает битые PDF лучше всего,
// а go-fitz тянет cgo и AGPL-лицензию MuPDF.
package pdftool

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"tutorgo/models"
)

var pageSizeRe = regexp.MustCompile(`(?m)^Page\s+\d+\s+size:\s+([\d.]+)\s+x\s+([\d.]+)\s+pts`)

// ParseInfo выбирает габариты страниц из вывода pdfinfo. Страницы идут по
// порядку — номер из строки не читаем, доверяем порядку вывода.
func ParseInfo(out string) ([]models.PageSizePt, error) {
	matches := pageSizeRe.FindAllStringSubmatch(out, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("pdfinfo: не нашёл ни одной страницы")
	}
	sizes := make([]models.PageSizePt, len(matches))
	for i, m := range matches {
		w, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return nil, err
		}
		h, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return nil, err
		}
		sizes[i] = models.PageSizePt{W: w, H: h}
	}
	return sizes, nil
}

// Info возвращает габариты всех страниц документа.
func Info(ctx context.Context, path string) ([]models.PageSizePt, error) {
	// -l с запасом: pdfinfo печатает страницы только в запрошенном диапазоне.
	out, err := exec.CommandContext(ctx, "pdfinfo", "-f", "1", "-l", "1000000", path).Output()
	if err != nil {
		return nil, fmt.Errorf("pdfinfo: %w", err)
	}
	return ParseInfo(string(out))
}

// RenderPage рендерит одну страницу (1-индексированную) в JPEG q85 и
// возвращает путь к файлу. Одна страница за вызов: память ограничена,
// а повтор джобы продолжает с места падения.
func RenderPage(ctx context.Context, pdfPath string, n, dpi int, outDir string) (string, error) {
	prefix := filepath.Join(outDir, fmt.Sprintf("p%d", n))
	cmd := exec.CommandContext(ctx, "pdftoppm",
		"-jpeg", "-jpegopt", "quality=85", "-r", strconv.Itoa(dpi),
		"-f", strconv.Itoa(n), "-l", strconv.Itoa(n), pdfPath, prefix)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftoppm стр. %d: %w: %s", n, err, out)
	}
	// pdftoppm сам паддит номер в имени (p1-1.jpg / p1-01.jpg) — ловим глобом.
	matches, err := filepath.Glob(prefix + "-*.jpg")
	if err != nil || len(matches) != 1 {
		return "", fmt.Errorf("pdftoppm стр. %d: ожидал 1 файл, получил %d", n, len(matches))
	}
	return matches[0], nil
}
```

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./pdftool/`
Expected: PASS.

- [ ] **Step 5: Smoke-тест с реальным poppler (скипается без бинарей)**

```go
// pdftool/smoke_test.go
package pdftool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Минимальный двухстраничный PDF без xref-таблицы: poppler реконструирует её
// сам (в stderr будет Syntax Warning — это норма). Вторая страница A4.
const tinyPdf = `%PDF-1.4
1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj
2 0 obj << /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >> endobj
3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >> endobj
4 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] >> endobj
trailer << /Root 1 0 R /Size 5 >>
`

func TestSmokePoppler(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm не установлен")
	}
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("pdfinfo не установлен")
	}
	dir := t.TempDir()
	pdf := filepath.Join(dir, "tiny.pdf")
	if err := os.WriteFile(pdf, []byte(tinyPdf), 0o644); err != nil {
		t.Fatal(err)
	}

	sizes, err := Info(context.Background(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	if len(sizes) != 2 || sizes[0].W != 612 || sizes[1].H != 842 {
		t.Fatalf("got %+v", sizes)
	}

	jpeg, err := RenderPage(context.Background(), pdf, 2, 72, dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(jpeg)
	if err != nil || st.Size() == 0 {
		t.Fatalf("пустой или отсутствующий jpeg: %v", err)
	}
}
```

- [ ] **Step 6: Прогнать всё**

Run: `go test ./pdftool/ -v`
Expected: PASS (или SKIP smoke, если poppler не стоит локально — тогда поставить: `sudo apt-get install poppler-utils` и перегнать).

- [ ] **Step 7: Commit**

```bash
git add pdftool/
git commit -m "feat: pdftool — pdfinfo/pdftoppm шелл-ауты с тестами"
```

---

### Task 3: River, роли в main.go, Dockerfile

**Files:**
- Create: `jobs/jobs.go`
- Modify: `main.go`
- Modify: `Dockerfile`
- Modify: `entrypoint.sh`
- Modify: `go.mod` (go get)

**Interfaces:**
- Produces (нужно задачам 4, 5):
  - `jobs.PdfImportArgs{ImportID string}` c `Kind() = "pdf_import"` и `InsertOpts()` (MaxAttempts 5)
  - `main.go`: ветка `ROLE=worker` зовёт `worker.Run(ctx, pool, cfg, log)` — сама функция появится в Task 5; до тех пор в main.go стоит заглушка с `log.Error` и `os.Exit(1)`
  - rivermigrate выполняется при старте обеих ролей

- [ ] **Step 1: Зависимости**

Run:
```bash
go get github.com/riverqueue/river@latest github.com/riverqueue/river/riverdriver/riverpgxv5@latest github.com/riverqueue/river/rivermigrate@latest
go mod tidy
```
Expected: go.mod пополнился, `go build ./...` чист.

- [ ] **Step 2: Тип джобы**

```go
// Package jobs — общие типы River-джоб: их вставляет API, исполняет воркер.
package jobs

import "github.com/riverqueue/river"

type PdfImportArgs struct {
	ImportID string `json:"import_id"`
}

func (PdfImportArgs) Kind() string { return "pdf_import" }

// 5 попыток с дефолтным backoff River (~секунды → минуты): транзиентные сбои
// S3/сети переживаем, битый файл не долбим вечно.
func (PdfImportArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 5}
}
```

- [ ] **Step 3: main.go — rivermigrate и ветка ролей**

В `main()` сразу после `pool := database.Connect(...)`:

```go
	// Схема River (river_job и служебные таблицы) — программной миграцией,
	// не через goose: у River свои версии. Идемпотентно, гоняется обеими ролями.
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		log.Error("river migrator", slog.String("error", err.Error()))
		os.Exit(1)
	}
	if _, err := migrator.Migrate(context.Background(), rivermigrate.DirectionUp, nil); err != nil {
		log.Error("river migrate", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if os.Getenv("ROLE") == "worker" {
		runWorker(pool, &cfg, log) // блокируется до SIGTERM
		return
	}
```

И в том же файле временная заглушка (Task 5 её заменит):

```go
// runWorker — роль worker: River-воркер PDF-импортов. Тело появится вместе с
// пакетом worker; до тех пор роль нерабочая.
func runWorker(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) {
	log.Error("worker role not implemented yet")
	os.Exit(1)
}
```

Импорты: `"github.com/riverqueue/river/riverdriver/riverpgxv5"`, `"github.com/riverqueue/river/rivermigrate"`, `"github.com/jackc/pgx/v5/pgxpool"`, `"tutorgo/config"`.

- [ ] **Step 4: Dockerfile и entrypoint**

Dockerfile, финальный образ:
```dockerfile
RUN apk add --no-cache ca-certificates netcat-openbsd poppler-utils
```

entrypoint.sh — воркер не гоняет goose (иначе гонка двух контейнеров на старте):
```sh
if [ "$ROLE" != "worker" ]; then
  ./goose -dir migrations postgres "$DB_URL" up
fi
exec ./main
```
(строка `./goose ... up` уже есть — обернуть её в это условие).

- [ ] **Step 5: Проверка**

Run: `go build ./... && go vet ./...`
Expected: чисто.

- [ ] **Step 6: Commit**

```bash
git add jobs/ main.go Dockerfile entrypoint.sh go.mod go.sum
git commit -m "feat: River в проекте — тип джобы, миграции, роль worker, poppler в образе"
```

---

### Task 4: Сервис импорта + тесты

**Files:**
- Create: `service/pdf_import.go`
- Create: `service/pdf_import_test.go`

**Interfaces:**
- Consumes: `repository.PdfImportRepository`, `models.*` (Task 1).
- Produces (нужно Task 7):
  - `service.PdfImportService` interface:
    - `CreateImport(ctx, boardID, pageID, tutorID, s3Key string, sizes []models.PageSizePt) (models.PdfPreflightResponse, error)`
    - `Start(ctx, importID, tutorID string, from, to int) (models.PdfStartResponse, error)`
  - `service.NewPdfImportService(repo repository.PdfImportRepository, enqueue func(ctx context.Context, tx pgx.Tx, importID string) error) PdfImportService`

- [ ] **Step 1: Падающие тесты**

По паттерну `service/whiteboard_test.go`: `package service_test`, testify/mock, мок репозитория прямо в файле теста.

```go
// service/pdf_import_test.go
package service_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"tutorgo/models"
	"tutorgo/service"
)

type mockPdfImportRepo struct{ mock.Mock }

func (m *mockPdfImportRepo) Create(ctx context.Context, boardID, pageID, tutorID, s3Key string, pages []models.PdfImportPage) (string, error) {
	args := m.Called(ctx, boardID, pageID, tutorID, s3Key, pages)
	return args.String(0), args.Error(1)
}
func (m *mockPdfImportRepo) GetByID(ctx context.Context, id string) (models.PdfImport, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.PdfImport), args.Error(1)
}
func (m *mockPdfImportRepo) Start(ctx context.Context, id string, from, to int, enqueue func(pgx.Tx) error) ([]models.PdfImportPage, error) {
	args := m.Called(ctx, id, from, to, enqueue)
	if err := enqueue(nil); err != nil { // убеждаемся, что enqueue-цепочка живая
		return nil, err
	}
	return args.Get(0).([]models.PdfImportPage), args.Error(1)
}
func (m *mockPdfImportRepo) MarkPageDone(ctx context.Context, id string, n int) error {
	return m.Called(ctx, id, n).Error(0)
}
func (m *mockPdfImportRepo) SetStatus(ctx context.Context, id, status string, errMsg *string) error {
	return m.Called(ctx, id, status, errMsg).Error(0)
}
func (m *mockPdfImportRepo) DeleteStalePending(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}
func (m *mockPdfImportRepo) BoardBelongsToTutor(ctx context.Context, boardID, tutorID string) (bool, error) {
	args := m.Called(ctx, boardID, tutorID)
	return args.Bool(0), args.Error(1)
}
func (m *mockPdfImportRepo) PageBelongsToBoard(ctx context.Context, pageID, boardID string) (bool, error) {
	args := m.Called(ctx, pageID, boardID)
	return args.Bool(0), args.Error(1)
}

func noEnqueue(ctx context.Context, tx pgx.Tx, importID string) error { return nil }

func TestCreateImportChecksOwnership(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("BoardBelongsToTutor", mock.Anything, "b1", "t1").Return(false, nil)

	_, err := svc.CreateImport(context.Background(), "b1", "p1", "t1", "key", []models.PageSizePt{{W: 612, H: 792}})
	assert.ErrorIs(t, err, service.ErrForbidden)
}

func TestCreateImportHappyPath(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("BoardBelongsToTutor", mock.Anything, "b1", "t1").Return(true, nil)
	repo.On("PageBelongsToBoard", mock.Anything, "p1", "b1").Return(true, nil)
	repo.On("Create", mock.Anything, "b1", "p1", "t1", "key",
		[]models.PdfImportPage{{N: 1, W: 612, H: 792}, {N: 2, W: 595, H: 842}},
	).Return("imp1", nil)

	resp, err := svc.CreateImport(context.Background(), "b1", "p1", "t1", "key",
		[]models.PageSizePt{{W: 612, H: 792}, {W: 595, H: 842}})
	assert.NoError(t, err)
	assert.Equal(t, "imp1", resp.ImportID)
	assert.Equal(t, 2, resp.NumPages)
	assert.Len(t, resp.PageSizes, 2)
}

func TestStartChecksOwnership(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("GetByID", mock.Anything, "imp1").Return(models.PdfImport{ID: "imp1", TutorID: "other"}, nil)

	_, err := svc.Start(context.Background(), "imp1", "t1", 1, 2)
	assert.ErrorIs(t, err, service.ErrForbidden)
}

func TestStartBuildsRelativeURLs(t *testing.T) {
	repo := new(mockPdfImportRepo)
	svc := service.NewPdfImportService(repo, noEnqueue)
	repo.On("GetByID", mock.Anything, "imp1").Return(models.PdfImport{ID: "imp1", TutorID: "t1"}, nil)
	repo.On("Start", mock.Anything, "imp1", 2, 3, mock.Anything).Return(
		[]models.PdfImportPage{
			{N: 2, AssetID: "a2", W: 612, H: 792},
			{N: 3, AssetID: "a3", W: 595, H: 842},
		}, nil)

	resp, err := svc.Start(context.Background(), "imp1", "t1", 2, 3)
	assert.NoError(t, err)
	assert.Len(t, resp.Pages, 2)
	assert.Equal(t, "a2", resp.Pages[0].FileID)
	assert.Equal(t, "/public/board-assets/a2", resp.Pages[0].URL)
	assert.Equal(t, 612.0, resp.Pages[0].W)
}
```

- [ ] **Step 2: Прогнать — падает**

Run: `go test ./service/ -run TestCreateImport -run TestStart`
Expected: FAIL (пакет не собирается: сервиса нет).

- [ ] **Step 3: Реализация**

```go
// service/pdf_import.go
package service

import (
	"context"

	"github.com/jackc/pgx/v5"

	"tutorgo/models"
	"tutorgo/repository"
)

type PdfImportService interface {
	// CreateImport — preflight: файл уже в S3, здесь ownership и запись pending.
	CreateImport(ctx context.Context, boardID, pageID, tutorID, s3Key string, sizes []models.PageSizePt) (models.PdfPreflightResponse, error)
	// Start фиксирует диапазон, создаёт ассеты и ставит джобу (транзакционно в repo).
	Start(ctx context.Context, importID, tutorID string, from, to int) (models.PdfStartResponse, error)
}

type pdfImportService struct {
	repo    repository.PdfImportRepository
	enqueue func(ctx context.Context, tx pgx.Tx, importID string) error
}

func NewPdfImportService(repo repository.PdfImportRepository, enqueue func(ctx context.Context, tx pgx.Tx, importID string) error) PdfImportService {
	return &pdfImportService{repo: repo, enqueue: enqueue}
}

func (s *pdfImportService) CreateImport(ctx context.Context, boardID, pageID, tutorID, s3Key string, sizes []models.PageSizePt) (models.PdfPreflightResponse, error) {
	ok, err := s.repo.BoardBelongsToTutor(ctx, boardID, tutorID)
	if err != nil {
		return models.PdfPreflightResponse{}, err
	}
	if !ok {
		return models.PdfPreflightResponse{}, ErrForbidden
	}
	ok, err = s.repo.PageBelongsToBoard(ctx, pageID, boardID)
	if err != nil {
		return models.PdfPreflightResponse{}, err
	}
	if !ok {
		return models.PdfPreflightResponse{}, ErrForbidden
	}

	pages := make([]models.PdfImportPage, len(sizes))
	for i, sz := range sizes {
		pages[i] = models.PdfImportPage{N: i + 1, W: sz.W, H: sz.H}
	}
	id, err := s.repo.Create(ctx, boardID, pageID, tutorID, s3Key, pages)
	if err != nil {
		return models.PdfPreflightResponse{}, err
	}
	return models.PdfPreflightResponse{ImportID: id, NumPages: len(sizes), PageSizes: sizes}, nil
}

func (s *pdfImportService) Start(ctx context.Context, importID, tutorID string, from, to int) (models.PdfStartResponse, error) {
	imp, err := s.repo.GetByID(ctx, importID)
	if err != nil {
		return models.PdfStartResponse{}, err
	}
	if imp.TutorID != tutorID {
		return models.PdfStartResponse{}, ErrForbidden
	}
	selected, err := s.repo.Start(ctx, importID, from, to, func(tx pgx.Tx) error {
		return s.enqueue(ctx, tx, importID)
	})
	if err != nil {
		return models.PdfStartResponse{}, err
	}
	out := make([]models.PdfImportPageOut, len(selected))
	for i, p := range selected {
		out[i] = models.PdfImportPageOut{
			FileID: p.AssetID,
			URL:    "/public/board-assets/" + p.AssetID,
			W:      p.W,
			H:      p.H,
		}
	}
	return models.PdfStartResponse{Pages: out}, nil
}
```

Если в `service/errors.go` нет `ErrForbidden` — добавить `var ErrForbidden = errors.New("forbidden")` (проверить существующие: возможно, уже есть аналог — тогда использовать его и поправить тесты).

- [ ] **Step 4: Прогнать тесты**

Run: `go test ./service/`
Expected: PASS, старые тесты тоже зелёные.

- [ ] **Step 5: Commit**

```bash
git add service/pdf_import.go service/pdf_import_test.go service/errors.go
git commit -m "feat: сервис PDF-импорта — preflight и start с ownership-проверками"
```

---

### Task 5: Воркер джобы + storage.Get + pg_notify

**Files:**
- Create: `worker/pdf_import.go`
- Create: `worker/pdf_import_test.go`
- Create: `worker/run.go`
- Modify: `storage/storage.go` (метод Get)
- Modify: `repository/pdf_import.go` (функция NotifyBoardEvent)
- Modify: `models/pdf_import.go` (тип BoardEvent)
- Modify: `main.go` (заменить заглушку runWorker)

**Interfaces:**
- Consumes: Task 1 (repo, models), Task 2 (pdftool), Task 3 (jobs, роль в main).
- Produces:
  - `models.BoardEvent{PageID string; Msg json.RawMessage}` — конверт pg_notify: `Msg` уходит в WS-хаб вербатим
  - `repository.NotifyBoardEvent(ctx, pool *pgxpool.Pool, ev models.BoardEvent) error`
  - `worker.Run(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) error`
  - канал NOTIFY называется `board_events` (Task 6 слушает его же)

- [ ] **Step 1: storage.Get**

```go
// Get скачивает объект целиком. Вызывающий закрывает reader.
func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}
```

- [ ] **Step 2: BoardEvent и NotifyBoardEvent**

В `models/pdf_import.go` добавить:

```go
// BoardEvent — конверт события для pg_notify('board_events'): адрес (страница
// доски) + готовое WS-сообщение, которое API-слушатель ретранслирует вербатим.
type BoardEvent struct {
	PageID string          `json:"page_id"`
	Msg    json.RawMessage `json:"msg"`
}
```

В `repository/pdf_import.go` добавить:

```go
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
```

- [ ] **Step 3: Падающий тест воркера**

Рендерер и заливка — за узкими func-полями, чтобы тест не требовал ни poppler, ни S3.

```go
// worker/pdf_import_test.go
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"tutorgo/jobs"
	"tutorgo/models"
)

// мок репозитория — те же сигнатуры, что в service/pdf_import_test.go
type mockRepo struct{ mock.Mock }

func (m *mockRepo) GetByID(ctx context.Context, id string) (models.PdfImport, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(models.PdfImport), args.Error(1)
}
func (m *mockRepo) MarkPageDone(ctx context.Context, id string, n int) error {
	return m.Called(ctx, id, n).Error(0)
}
func (m *mockRepo) SetStatus(ctx context.Context, id, status string, errMsg *string) error {
	return m.Called(ctx, id, status, errMsg).Error(0)
}

func testImport() models.PdfImport {
	return models.PdfImport{
		ID: "imp1", PageID: "page1", S3Key: "board-pdf/x.pdf", Status: "rendering",
		Pages: []models.PdfImportPage{
			{N: 1, AssetID: "a1", W: 612, H: 792, Done: true}, // уже готова: повтор джобы
			{N: 2, AssetID: "a2", W: 612, H: 792},
		},
	}
}

func newTestWorker(repo *mockRepo, events *[]models.BoardEvent, renderErr error) *PdfImportWorker {
	return &PdfImportWorker{
		Repo: repo,
		Fetch: func(ctx context.Context, key string) (string, error) { return "/tmp/fake.pdf", nil },
		Render: func(ctx context.Context, pdfPath string, n int) (string, error) {
			if renderErr != nil {
				return "", renderErr
			}
			return "/tmp/fake.jpg", nil
		},
		Upload: func(ctx context.Context, key, jpegPath string) error { return nil },
		Notify: func(ctx context.Context, ev models.BoardEvent) error {
			*events = append(*events, ev)
			return nil
		},
	}
}

func job(attempt, maxAttempts int) *river.Job[jobs.PdfImportArgs] {
	return &river.Job[jobs.PdfImportArgs]{
		JobRow: &rivertype.JobRow{Attempt: attempt, MaxAttempts: maxAttempts},
		Args:   jobs.PdfImportArgs{ImportID: "imp1"},
	}
}

func TestWorkSkipsDonePagesAndNotifies(t *testing.T) {
	repo := new(mockRepo)
	var events []models.BoardEvent
	w := newTestWorker(repo, &events, nil)
	repo.On("GetByID", mock.Anything, "imp1").Return(testImport(), nil)
	repo.On("MarkPageDone", mock.Anything, "imp1", 2).Return(nil)
	repo.On("SetStatus", mock.Anything, "imp1", "done", (*string)(nil)).Return(nil)

	err := w.Work(context.Background(), job(1, 5))
	assert.NoError(t, err)
	// страница 1 done — не трогали; страница 2 отрендерена и объявлена
	repo.AssertNotCalled(t, "MarkPageDone", mock.Anything, "imp1", 1)
	assert.Len(t, events, 1)
	assert.Equal(t, "page1", events[0].PageID)
	assert.True(t, strings.Contains(string(events[0].Msg), `"a2"`))
	assert.True(t, strings.Contains(string(events[0].Msg), `"file"`))
}

func TestWorkFinalAttemptMarksFailed(t *testing.T) {
	repo := new(mockRepo)
	var events []models.BoardEvent
	w := newTestWorker(repo, &events, errors.New("битый pdf"))
	repo.On("GetByID", mock.Anything, "imp1").Return(testImport(), nil)
	repo.On("SetStatus", mock.Anything, "imp1", "failed", mock.Anything).Return(nil)

	err := w.Work(context.Background(), job(5, 5)) // последняя попытка
	assert.Error(t, err)
	repo.AssertCalled(t, "SetStatus", mock.Anything, "imp1", "failed", mock.Anything)
	// import_failed с fileId нерендерённых страниц
	assert.Len(t, events, 1)
	var ev struct {
		Type    string `json:"type"`
		Payload struct {
			FileIDs []string `json:"fileIds"`
		} `json:"payload"`
	}
	assert.NoError(t, json.Unmarshal(events[0].Msg, &ev))
	assert.Equal(t, "import_failed", ev.Type)
	assert.Equal(t, []string{"a2"}, ev.Payload.FileIDs)
}

func TestWorkTransientErrorRetries(t *testing.T) {
	repo := new(mockRepo)
	var events []models.BoardEvent
	w := newTestWorker(repo, &events, errors.New("s3 моргнул"))
	repo.On("GetByID", mock.Anything, "imp1").Return(testImport(), nil)

	err := w.Work(context.Background(), job(2, 5)) // не последняя попытка
	assert.Error(t, err)
	repo.AssertNotCalled(t, "SetStatus", mock.Anything, "imp1", "failed", mock.Anything)
	assert.Empty(t, events)
}
```

Импорт `rivertype`: `"github.com/riverqueue/river/rivertype"`.

- [ ] **Step 4: Прогнать — падает**

Run: `go test ./worker/`
Expected: FAIL (пакета нет).

- [ ] **Step 5: Реализация воркера**

```go
// Package worker — роль worker: River-джобы PDF-импорта.
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/riverqueue/river"

	"tutorgo/jobs"
	"tutorgo/models"
)

// pdfImportRepo — срез PdfImportRepository, нужный джобе (узкий интерфейс
// ради тестов без БД).
type pdfImportRepo interface {
	GetByID(ctx context.Context, id string) (models.PdfImport, error)
	MarkPageDone(ctx context.Context, id string, n int) error
	SetStatus(ctx context.Context, id, status string, errMsg *string) error
}

type PdfImportWorker struct {
	river.WorkerDefaults[jobs.PdfImportArgs]
	Repo pdfImportRepo
	// Инъекции вместо прямых вызовов storage/pdftool: джоба тестируется без S3
	// и poppler. Реальные реализации собирает Run().
	Fetch  func(ctx context.Context, s3Key string) (localPath string, err error)
	Render func(ctx context.Context, pdfPath string, n int) (jpegPath string, err error)
	Upload func(ctx context.Context, s3Key, jpegPath string) error
	Notify func(ctx context.Context, ev models.BoardEvent) error
	Log    *slog.Logger
}

// fileMsg — WS-сообщение типа 'file', тот же формат, что клиентский registerFile.
func fileMsg(fileID, url string) json.RawMessage {
	msg, _ := json.Marshal(map[string]any{
		"type": "file",
		"payload": map[string]string{
			"fileId":   fileID,
			"url":      url,
			"mimeType": "image/jpeg",
		},
	})
	return msg
}

func failMsg(fileIDs []string) json.RawMessage {
	msg, _ := json.Marshal(map[string]any{
		"type":    "import_failed",
		"payload": map[string]any{"fileIds": fileIDs},
	})
	return msg
}

func (w *PdfImportWorker) Work(ctx context.Context, job *river.Job[jobs.PdfImportArgs]) error {
	imp, err := w.Repo.GetByID(ctx, job.Args.ImportID)
	if err != nil {
		return err
	}
	if imp.Status == "done" || imp.Status == "failed" {
		return nil // гонка повторов — уже отработано
	}

	err = w.renderAll(ctx, imp)
	if err == nil {
		return w.Repo.SetStatus(ctx, imp.ID, "done", nil)
	}

	// Последняя попытка: фиксируем провал и говорим клиентам убрать
	// плейсхолдеры нерендерённых страниц. Ошибки этой ветки не важнее исходной.
	if job.Attempt >= job.MaxAttempts {
		msg := err.Error()
		_ = w.Repo.SetStatus(ctx, imp.ID, "failed", &msg)
		var pending []string
		for _, p := range imp.Pages {
			if !p.Done {
				pending = append(pending, p.AssetID)
			}
		}
		_ = w.Notify(ctx, models.BoardEvent{PageID: imp.PageID, Msg: failMsg(pending)})
	}
	return err
}

func (w *PdfImportWorker) renderAll(ctx context.Context, imp models.PdfImport) error {
	pdfPath, err := w.Fetch(ctx, imp.S3Key)
	if err != nil {
		return fmt.Errorf("fetch original: %w", err)
	}
	defer os.Remove(pdfPath)

	for _, p := range imp.Pages {
		if p.Done {
			continue // идемпотентность: повтор джобы не перерендеривает готовое
		}
		jpegPath, err := w.Render(ctx, pdfPath, p.N)
		if err != nil {
			return fmt.Errorf("render page %d: %w", p.N, err)
		}
		key := models.PdfPageAssetKey(imp.ID, p.N)
		if err := w.Upload(ctx, key, jpegPath); err != nil {
			return fmt.Errorf("upload page %d: %w", p.N, err)
		}
		if err := w.Repo.MarkPageDone(ctx, imp.ID, p.N); err != nil {
			return err
		}
		// Уведомление best-effort: если NOTIFY потерялся, клиент увидит страницу
		// после reload (URL уже в files-карте снапшота).
		if err := w.Notify(ctx, models.BoardEvent{
			PageID: imp.PageID,
			Msg:    fileMsg(p.AssetID, "/public/board-assets/"+p.AssetID),
		}); err != nil && w.Log != nil {
			w.Log.Warn("pdf import notify", slog.String("error", err.Error()))
		}
	}
	return nil
}
```

- [ ] **Step 6: Прогнать тесты воркера**

Run: `go test ./worker/`
Expected: PASS.

- [ ] **Step 7: worker.Run и main.go**

```go
// worker/run.go
package worker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"tutorgo/config"
	"tutorgo/models"
	"tutorgo/pdftool"
	"tutorgo/repository"
	"tutorgo/storage"
)

const renderDPI = 200

// Run поднимает River-воркер и блокируется до отмены ctx.
func Run(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) error {
	store, err := storage.New(ctx, *cfg)
	if err != nil {
		return err
	}
	repo := repository.NewPdfImportRepository(pool)

	pdfWorker := &PdfImportWorker{
		Repo: repo,
		Fetch: func(ctx context.Context, s3Key string) (string, error) {
			body, err := store.Get(ctx, s3Key)
			if err != nil {
				return "", err
			}
			defer body.Close()
			tmp, err := os.CreateTemp("", "pdfimport-*.pdf")
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(tmp, body); err != nil {
				tmp.Close()
				os.Remove(tmp.Name())
				return "", err
			}
			return tmp.Name(), tmp.Close()
		},
		Render: func(ctx context.Context, pdfPath string, n int) (string, error) {
			dir, err := os.MkdirTemp("", "pdfpages-*")
			if err != nil {
				return "", err
			}
			// Директорию не чистим сразу — файл нужен Upload'у; уборка ниже в Upload.
			return pdftool.RenderPage(ctx, pdfPath, n, renderDPI, dir)
		},
		Upload: func(ctx context.Context, s3Key, jpegPath string) error {
			f, err := os.Open(jpegPath)
			if err != nil {
				return err
			}
			defer func() {
				f.Close()
				// Убираем всю per-render временную директорию (см. Render: MkdirTemp
				// на каждую страницу), иначе /tmp воркера засоряется пустыми папками.
				os.RemoveAll(filepath.Dir(jpegPath))
			}()
			st, err := f.Stat()
			if err != nil {
				return err
			}
			return store.Put(ctx, s3Key, f, st.Size(), "image/jpeg")
		},
		Notify: func(ctx context.Context, ev models.BoardEvent) error {
			return repository.NotifyBoardEvent(ctx, pool, ev)
		},
		Log: log,
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, pdfWorker)
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 2}},
		Workers: workers,
	})
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}
	log.Info("pdf import worker started")

	// Чистка протухших preflight: раз в час, плюс сразу при старте.
	go func() {
		t := time.NewTicker(time.Hour)
		defer t.Stop()
		for {
			keys, err := repo.DeleteStalePending(ctx)
			if err != nil {
				log.Error("cleanup stale pdf imports", slog.String("error", err.Error()))
			}
			for _, k := range keys {
				if err := store.Remove(ctx, k); err != nil {
					log.Warn("remove stale pdf", slog.String("key", k), slog.String("error", err.Error()))
				}
			}
			select {
			case <-t.C:
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		return fmt.Errorf("river stop: %w", err)
	}
	return nil
}
```

В `main.go` заменить заглушку:

```go
func runWorker(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := worker.Run(ctx, pool, cfg, log); err != nil {
		log.Error("worker", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("worker exited cleanly")
}
```

- [ ] **Step 8: Проверка**

Run: `go build ./... && go test ./worker/ ./service/ ./pdftool/`
Expected: всё зелёное.

- [ ] **Step 9: Commit**

```bash
git add worker/ storage/storage.go repository/pdf_import.go models/pdf_import.go main.go
git commit -m "feat: воркер PDF-импорта — рендер, заливка, pg_notify, чистка protухших"
```

---

### Task 6: LISTEN-мост и PushToPage

**Files:**
- Modify: `handlers/whiteboard_ws.go` (метод PushToPage)
- Create: `handlers/board_events.go`
- Modify: `router/router.go` (вернуть менеджер хабов наружу)
- Modify: `main.go` (запуск слушателя)

**Interfaces:**
- Consumes: `models.BoardEvent` (формат описан в Task 5; при параллельной работе тип может ещё отсутствовать — тогда объявить его в models по сигнатуре из Task 5, git смержит).
- Produces:
  - `(*WbHubManager).PushToPage(pageID string, data []byte)`
  - `handlers.ListenBoardEvents(ctx context.Context, pool *pgxpool.Pool, mgr *WbHubManager, log *slog.Logger)`
  - `router.Setup` возвращает третьим значением `*handlers.WbHubManager`

- [ ] **Step 1: PushToPage**

В `handlers/whiteboard_ws.go`:

```go
// PushToPage вбрасывает серверное сообщение в хаб страницы. Если хаба нет —
// никто не подключён, и слать некому: молча выходим (клиент увидит контент из
// снапшота/S3 при следующем подключении). sender=nil ⇒ run() раздаст всем.
// ВАЖНО: только для типов, которые run() ретранслирует вербатим (default-ветка);
// cursor/viewport разыменовывают sender и с nil упадут.
func (m *WbHubManager) PushToPage(pageID string, data []byte) {
	m.mu.Lock()
	hub, ok := m.hubs[pageID]
	m.mu.Unlock()
	if !ok {
		return
	}
	select {
	case hub.broadcast <- wbBroadcast{sender: nil, data: data}:
	default:
		// Переполненный канал — не повод блокировать слушателя NOTIFY.
		m.log.Warn("board hub broadcast full, drop server event", slog.String("pageId", pageID))
	}
}
```

(проверить импорт `log/slog` в файле; поле log у менеджера уже есть).

- [ ] **Step 2: Слушатель**

```go
// handlers/board_events.go
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tutorgo/models"
)

// ListenBoardEvents держит выделенное соединение с LISTEN board_events и
// пересылает события воркера в WS-хабы. Блокируется до отмены ctx; при обрыве
// соединения переподключается с паузой.
func ListenBoardEvents(ctx context.Context, pool *pgxpool.Pool, mgr *WbHubManager, log *slog.Logger) {
	for ctx.Err() == nil {
		if err := listenOnce(ctx, pool, mgr); err != nil && ctx.Err() == nil {
			log.Warn("board events listener", slog.String("error", err.Error()))
			time.Sleep(3 * time.Second)
		}
	}
}

func listenOnce(ctx context.Context, pool *pgxpool.Pool, mgr *WbHubManager) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// Соединение испорчено LISTEN-состоянием — в пул его не возвращаем.
	defer conn.Conn().Close(context.Background()) //nolint:errcheck
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN board_events"); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var ev models.BoardEvent
		if err := json.Unmarshal([]byte(n.Payload), &ev); err != nil || ev.PageID == "" {
			continue // мусор в канале — не наш, пропускаем
		}
		mgr.PushToPage(ev.PageID, ev.Msg)
	}
}
```

- [ ] **Step 3: Wiring**

`router/router.go`: сигнатура `Setup(...) (*gin.Engine, service.SubscriptionService)` → добавить третье возвращаемое `*handlers.WbHubManager` (вернуть `wbHubManager`, он уже создаётся на строке ~82). Поправить вызов в `main.go`:

```go
	r, subscriptionService, wbHubManager := router.Setup(pool, log, &cfg)
```

И в блок фоновых горутин `main.go` (после существующих `bgWg.Go`):

```go
	bgWg.Go(func() {
		handlers.ListenBoardEvents(bgCtx, pool, wbHubManager, log)
	})
```

- [ ] **Step 4: Проверка**

Run: `go build ./... && go test ./handlers/ && go vet ./...`
Expected: чисто, существующие тесты хендлеров зелёные.

- [ ] **Step 5: Commit**

```bash
git add handlers/whiteboard_ws.go handlers/board_events.go router/router.go main.go
git commit -m "feat: LISTEN board_events → WS-хабы досок (PushToPage)"
```

---

### Task 7: HTTP-хендлеры и роуты

**Files:**
- Create: `handlers/pdf_import.go`
- Modify: `router/router.go`
- Modify: `handlers/mocks_test.go` (мок PdfImportService, если понадобится в тестах хендлеров)

**Interfaces:**
- Consumes: `service.PdfImportService` (Task 4), `pdftool.Info` (Task 2), `storage.Client.Put`, `jobs.PdfImportArgs` + insert-only River-клиент (Task 3).
- Produces (контракт для Task 8):
  - `POST /boards/:boardId/pdf` — multipart: `file` (PDF ≤ 50 МБ), `page_id` (uuid страницы доски). Ответ 200: `models.PdfPreflightResponse` (`{import_id, num_pages, page_sizes:[{w,h}]}`, размеры в pt)
  - `POST /pdf-imports/:id/start` — JSON `{from, to}` (1-индексированные, включительно). Ответ 200: `models.PdfStartResponse` (`{pages:[{file_id, url, w, h}]}`, url относительный)

- [ ] **Step 1: Хендлер**

```go
// handlers/pdf_import.go
package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tutorgo/models"
	"tutorgo/pdftool"
	"tutorgo/service"
	"tutorgo/storage"
)

type PdfImportHandler struct {
	svc   service.PdfImportService
	store *storage.Client
	log   *slog.Logger
	// infoFn — pdftool.Info за полем: тесты подставляют фейк, exec не нужен.
	infoFn func(ctx context.Context, path string) ([]models.PageSizePt, error)
}

func NewPdfImportHandler(svc service.PdfImportService, store *storage.Client, log *slog.Logger) *PdfImportHandler {
	return &PdfImportHandler{svc: svc, store: store, log: log, infoFn: pdftool.Info}
}

const maxPdfBytes = 50 << 20 // держать в синхроне с MAX_ASSET_BYTES фронта

// Upload — preflight: оригинал в S3, метаданные страниц клиенту.
// POST /boards/:boardId/pdf (multipart: file, page_id)
func (h *PdfImportHandler) Upload(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	pageID := c.PostForm("page_id")
	if pageID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page_id required"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
		return
	}
	defer file.Close()
	if header.Size > maxPdfBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large (max 50MB)"})
		return
	}

	// pdfinfo работает с путём — кладём во временный файл.
	tmp, err := os.CreateTemp("", "pdfupload-*.pdf")
	if err != nil {
		h.log.Error("pdf upload: temp", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer os.Remove(tmp.Name())
	if err := c.SaveUploadedFile(header, tmp.Name()); err != nil {
		h.log.Error("pdf upload: save", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	sizes, err := h.infoFn(c.Request.Context(), tmp.Name())
	if err != nil {
		h.log.Warn("pdf upload: pdfinfo", "err", err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "не удалось прочитать PDF"})
		return
	}

	src, err := os.Open(tmp.Name())
	if err != nil {
		h.log.Error("pdf upload: reopen", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer src.Close()
	key := fmt.Sprintf("board-pdf/%s%s", uuid.New().String(), filepath.Ext(header.Filename))
	if err := h.store.Put(c.Request.Context(), key, src, header.Size, "application/pdf"); err != nil {
		h.log.Error("pdf upload: s3", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}

	resp, err := h.svc.CreateImport(c.Request.Context(), c.Param("boardId"), pageID, tutorID, key, sizes)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		h.log.Error("pdf upload: create import", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Start фиксирует диапазон и ставит джобу.
// POST /pdf-imports/:id/start {from, to}
func (h *PdfImportHandler) Start(c *gin.Context) {
	tutorID := c.GetString("tutorID")
	if tutorID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req models.StartPdfImportRequest
	if !bindAndValidate(c, &req) {
		return
	}
	resp, err := h.svc.Start(c.Request.Context(), c.Param("id"), tutorID, req.From, req.To)
	if err != nil {
		if errors.Is(err, service.ErrForbidden) {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		h.log.Warn("pdf import start", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
```

- [ ] **Step 2: Wiring в router.go**

Рядом с существующим whiteboard-wiring (строки ~36, 55, 82):

```go
	pdfImportRepo := repository.NewPdfImportRepository(pool)
	// Insert-only River-клиент: воркеров в API-роли нет, только постановка джоб.
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		log.Error("river client", slog.String("error", err.Error()))
		os.Exit(1)
	}
	pdfImportService := service.NewPdfImportService(pdfImportRepo,
		func(ctx context.Context, tx pgx.Tx, importID string) error {
			_, err := riverClient.InsertTx(ctx, tx, jobs.PdfImportArgs{ImportID: importID}, nil)
			return err
		})
	pdfImportHandler := handlers.NewPdfImportHandler(pdfImportService, store, log)
```

Роуты в защищённой группе (рядом с `auth.POST("/boards/:boardId/assets", ...)`):

```go
		auth.POST("/boards/:boardId/pdf", pdfImportHandler.Upload)
		auth.POST("/pdf-imports/:id/start", pdfImportHandler.Start)
```

- [ ] **Step 3: Проверка**

Run: `go build ./... && go test ./... 2>&1 | tail -20`
Expected: сборка и все тесты зелёные.

- [ ] **Step 4: Ручной smoke (если есть локальная БД и .env)**

```bash
make run &
# staged: залить любой pdf
curl -s -X POST localhost:8080/boards/<boardId>/pdf \
  -H "Authorization: Bearer <tutor JWT>" \
  -F file=@some.pdf -F page_id=<pageId> | jq
```
Expected: `{"import_id": "...", "num_pages": N, "page_sizes": [...]}`.

- [ ] **Step 5: Commit**

```bash
git add handlers/pdf_import.go router/router.go
git commit -m "feat: роуты PDF-импорта — preflight и start"
```

---

### Task 8: Фронтенд — новый поток, удаление pdf.js

**Files:**
- Modify: `frontend/src/lib/api/whiteboard.ts`
- Modify: `frontend/src/types/api.ts`
- Modify: `frontend/src/components/whiteboard/ExcalidrawCanvas.tsx`
- Modify: `frontend/src/components/whiteboard/useExcalidrawSync.ts`
- Delete: `frontend/src/lib/pdf.ts`
- Modify: `frontend/package.json` (убрать pdfjs-dist)

**Interfaces:**
- Consumes: контракт API из Task 7; `layoutPages` из `@/lib/pdfRange` (есть, покрыт тестами); pending-элементы и `registerFile` (есть).
- Produces: —

- [ ] **Step 1: Типы и API-клиент**

В `frontend/src/types/api.ts`:

```ts
export interface PdfPreflightResponse {
  import_id: string
  num_pages: number
  page_sizes: { w: number; h: number }[] // пункты PDF
}

export interface PdfImportPageOut {
  file_id: string
  url: string // относительный, префиксуем BASE_URL
  w: number
  h: number
}

export interface PdfStartResponse {
  pages: PdfImportPageOut[]
}
```

В `whiteboardApi` (`frontend/src/lib/api/whiteboard.ts`):

```ts
  // Preflight: оригинал PDF уезжает на сервер, обратно — паспорт документа.
  uploadPdf: (boardId: string, pageId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    form.append('page_id', pageId)
    return api
      .post<PdfPreflightResponse>(`/boards/${boardId}/pdf`, form, {
        headers: { 'Content-Type': false }, // boundary выставит браузер
      })
      .then((r) => r.data)
  },

  startPdfImport: (importId: string, from: number, to: number) =>
    api
      .post<PdfStartResponse>(`/pdf-imports/${importId}/start`, { from, to })
      .then((r) => r.data),
```

(импортировать типы из `@/types/api`).

- [ ] **Step 2: useExcalidrawSync — относительные URL и import_failed**

В `hydrateFiles` (fetch файла): относительный URL от воркера префиксуем:

```ts
          const src = meta.url.startsWith('/') ? `${BASE_URL}${meta.url}` : meta.url
          const resp = await fetch(src)
```

(импорт: `import { getWsUrl, BASE_URL } from '@/lib/api/whiteboard'`).

В `ws.onmessage` новая ветка после `if (msg.type === 'file') {...}`:

```ts
      // Сервер не смог отрендерить PDF: убираем плейсхолдеры навсегда —
      // файла для них не будет. Тумбстоуны уедут пирам обычным диффом.
      if (msg.type === 'import_failed') {
        const p = msg.payload as { fileIds?: string[] }
        const dead = new Set(p?.fileIds ?? [])
        const api = apiRef.current
        if (!api || dead.size === 0) return
        api.updateScene({
          elements: api
            .getSceneElementsIncludingDeleted()
            .map((el) =>
              el.type === 'image' && el.fileId && dead.has(el.fileId)
                ? newElementWith(el, { isDeleted: true })
                : el
            ),
          captureUpdate: CaptureUpdateAction.IMMEDIATELY,
        })
        onImportFailedRef.current?.(p?.fileIds ?? [])
        return
      }
```

Импорт `newElementWith` из `@excalidraw/excalidraw`. Колбэки наружу — по образцу `onMediaRef`:

```ts
// в сигнатуре хука: onFile?: (fileId: string) => void, onImportFailed?: (fileIds: string[]) => void
const onFileRef = useRef(onFile)
const onImportFailedRef = useRef(onImportFailed)
useEffect(() => {
  onFileRef.current = onFile
  onImportFailedRef.current = onImportFailed
}, [onFile, onImportFailed])
```

И в существующей ветке `msg.type === 'file'` после `hydrateFiles(...)` добавить `onFileRef.current?.(p.fileId)`.

- [ ] **Step 3: ExcalidrawCanvas — новый поток**

Удалить: импорт `loadPdf, pageSize, renderPage` и `PDFDocumentProxy`, `pdfRef`, константу `PAGE_CONCURRENCY`, всё тело старого `handlePdfConfirm` (рендер-пул).

Состояние вместо `pdfRef`:

```ts
  // Паспорт preflight'а: PDF уже на сервере, ждём выбора диапазона.
  const pdfImportRef = useRef<{ importId: string; sizes: { w: number; h: number }[] } | null>(null)
```

Drop-обработчик PDF (там, где был `loadPdf`):

```ts
      try {
        const preflight = await whiteboardApi.uploadPdf(boardId, page!.id, file)
        pdfImportRef.current = { importId: preflight.import_id, sizes: preflight.page_sizes }
        setPdfDialog({ numPages: preflight.num_pages, point })
      } catch {
        toast.error('Не удалось открыть PDF')
      }
```

Прогресс: набор ожидаемых fileId живёт в ref; колбэк из хука его тратит:

```ts
  // Ожидаемые страницы текущего импорта: file-события с этими id двигают тост.
  const pdfProgressRef = useRef<{ pending: Set<string>; total: number; toastId: string } | null>(null)

  const onPdfFile = useCallback((fileId: string) => {
    const pr = pdfProgressRef.current
    if (!pr || !pr.pending.delete(fileId)) return
    const done = pr.total - pr.pending.size
    if (pr.pending.size === 0) {
      toast.success(`PDF вставлен: ${pr.total} стр.`, { id: pr.toastId })
      pdfProgressRef.current = null
    } else {
      toast.loading(`PDF: ${done} / ${pr.total}…`, { id: pr.toastId })
    }
  }, [])

  const onPdfFailed = useCallback((fileIds: string[]) => {
    const pr = pdfProgressRef.current
    if (pr && fileIds.some((id) => pr.pending.has(id))) {
      toast.error('Не удалось обработать PDF', { id: pr.toastId })
      pdfProgressRef.current = null
    }
  }, [])
```

Передать в хук: `useExcalidrawSync(page, token, identity, onMedia, onPdfFile, onPdfFailed)` (сигнатуру хука расширить соответственно).

Новый `handlePdfConfirm`:

```ts
  // Диапазон выбран: сервер ставит джобу, мы раскладываем плейсхолдеры.
  // Картинки проявятся file-событиями по мере рендера на сервере.
  const handlePdfConfirm = (from: number, to: number) => {
    const imp = pdfImportRef.current
    const origin = pdfDialog?.point
    if (!imp || !origin) return
    pdfImportRef.current = null
    setPdfDialog(null)

    const toastId = `pdf-${Date.now()}`
    toast.loading('PDF: готовим страницы…', { id: toastId })

    void (async () => {
      const api = apiRef.current
      if (!api) return
      try {
        const { pages } = await whiteboardApi.startPdfImport(imp.importId, from, to)
        // Сервер отдаёт пункты PDF — масштаб сцены наш.
        const LAYOUT_SCALE = 1.5
        const placed = layoutPages(
          pages.map((p) => ({ w: p.w * LAYOUT_SCALE, h: p.h * LAYOUT_SCALE })),
          origin
        )
        const els = convertToExcalidrawElements(
          placed.map((l, k) => ({
            type: 'image' as const,
            fileId: pages[k].file_id as FileId,
            x: l.x,
            y: l.y,
            width: l.w,
            height: l.h,
            status: 'pending' as const,
          }))
        )
        api.updateScene({
          elements: [...api.getSceneElementsIncludingDeleted(), ...els],
          captureUpdate: CaptureUpdateAction.IMMEDIATELY,
        })
        // Указатели fileId→URL сразу в files-карту и пирам: раскладка и ссылки
        // переживают закрытие вкладки — сервер дорендерит без нас.
        for (const p of pages) {
          registerFile(p.file_id, `${BASE_URL}${p.url}`, 'image/jpeg')
        }
        pdfProgressRef.current = {
          pending: new Set(pages.map((p) => p.file_id)),
          total: pages.length,
          toastId,
        }
        toast.loading(`PDF: 0 / ${pages.length}…`, { id: toastId })
      } catch (e) {
        const msg = (e as { message?: string })?.message
        toast.error(`Не удалось обработать PDF${msg ? `: ${msg}` : ''}`, { id: toastId })
      }
    })()
  }
```

`layoutPages` уже импортирован из `@/lib/pdfRange`, `BASE_URL` — из `@/lib/api/whiteboard`.

- [ ] **Step 4: Удаления**

```bash
cd frontend
git rm src/lib/pdf.ts
npm uninstall pdfjs-dist
```
Проверить, что `imageFromClipboard`/drop-обработчик не ссылаются на удалённое; PDF-ветка drop'а остаётся (files с типом `application/pdf` перехватываются и уходят в uploadPdf).

- [ ] **Step 5: Проверка**

Run: `cd frontend && npx tsc --noEmit && node --test --experimental-strip-types "src/**/*.test.ts"`
Expected: типы чистые, все тесты зелёные (тесты `pdfRange` не тронуты).

- [ ] **Step 6: Ручной smoke (два браузера)**

1. Открыть доску преподом, дропнуть многостраничный PDF → диалог появляется быстро, с числом страниц.
2. Подтвердить диапазон → плейсхолдеры встают сразу (7 в строке), страницы проявляются, тост считает.
3. Второй браузер (ученик по ссылке) видит те же страницы по мере готовности.
4. Закрыть вкладку препода посреди импорта, перезагрузить ученика → все страницы на месте.

- [ ] **Step 7: Commit**

```bash
git add -A frontend
git commit -m "feat: PDF на доску через серверный импорт, pdf.js удалён из бандла"
```

---

## Self-review

- Каждый пункт спеки покрыт: preflight (T7), очередь River (T3, T5), pdftoppm (T2), NOTIFY→хаб (T5, T6), плейсхолдеры/раскладка/registerFile (T8), import_failed (T5, T6, T8), чистка pending (T5), удаление pdf.js (T8), Dockerfile/роль (T3).
- Типы согласованы: `models.PdfImportPage/PdfImport/BoardEvent/PageSizePt` едины между задачами; ключ S3 страницы — только через `models.PdfPageAssetKey`.
- Канал NOTIFY назван `board_events` в T5 и T6 одинаково.

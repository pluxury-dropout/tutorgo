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

	pages, err := w.renderAll(ctx, imp)
	if err == nil {
		return w.Repo.SetStatus(ctx, imp.ID, "done", nil)
	}

	// Последняя попытка: фиксируем провал и говорим клиентам убрать
	// плейсхолдеры нерендерённых страниц. Ошибки этой ветки не важнее исходной.
	if job.Attempt >= job.MaxAttempts {
		msg := err.Error()
		_ = w.Repo.SetStatus(ctx, imp.ID, "failed", &msg)
		var pending []string
		for _, p := range pages {
			if !p.Done {
				pending = append(pending, p.AssetID)
			}
		}
		if err := w.Notify(ctx, models.BoardEvent{PageID: imp.PageID, Msg: failMsg(pending)}); err != nil && w.Log != nil {
			w.Log.Warn("pdf import notify failed", slog.String("error", err.Error()))
		}
	}
	return err
}

// renderAll рендерит недостающие страницы и возвращает актуальный список
// страниц импорта: страницы, успешно отрендеренные и загруженные в этой же
// попытке, помечаются Done=true в возвращаемой копии — вызывающий код (ветка
// последней попытки в Work) должен ориентироваться на неё, а не на снапшот
// imp.Pages, прочитанный до рендера, иначе только что готовые страницы
// попадут в список «провалившихся».
func (w *PdfImportWorker) renderAll(ctx context.Context, imp models.PdfImport) ([]models.PdfImportPage, error) {
	pages := append([]models.PdfImportPage(nil), imp.Pages...)

	pdfPath, err := w.Fetch(ctx, imp.S3Key)
	if err != nil {
		return pages, fmt.Errorf("fetch original: %w", err)
	}
	defer os.Remove(pdfPath)

	for i := range pages {
		p := &pages[i]
		if p.Done {
			continue // идемпотентность: повтор джобы не перерендеривает готовое
		}
		jpegPath, err := w.Render(ctx, pdfPath, p.N)
		if err != nil {
			return pages, fmt.Errorf("render page %d: %w", p.N, err)
		}
		key := models.PdfPageAssetKey(imp.ID, p.N)
		if err := w.Upload(ctx, key, jpegPath); err != nil {
			return pages, fmt.Errorf("upload page %d: %w", p.N, err)
		}
		if err := w.Repo.MarkPageDone(ctx, imp.ID, p.N); err != nil {
			return pages, err
		}
		p.Done = true
		// Уведомление best-effort: если NOTIFY потерялся, клиент увидит страницу
		// после reload (URL уже в files-карте снапшота).
		if err := w.Notify(ctx, models.BoardEvent{
			PageID: imp.PageID,
			Msg:    fileMsg(p.AssetID, "/public/board-assets/"+p.AssetID),
		}); err != nil && w.Log != nil {
			w.Log.Warn("pdf import notify", slog.String("error", err.Error()))
		}
	}
	return pages, nil
}

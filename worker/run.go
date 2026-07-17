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

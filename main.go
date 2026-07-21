package main

import (
	"context"
	"embed"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"tutorgo/config"
	"tutorgo/database"
	"tutorgo/handlers"
	"tutorgo/repository"
	"tutorgo/router"
	"tutorgo/worker"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// Миграции вкомпилированы в бинарник: образу не нужны ни каталог migrations/,
// ни бинарник goose, и SQL физически не может разъехаться с кодом.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

func runIntervalLoop(ctx context.Context, interval time.Duration, name string, job func(context.Context) (int64, error), log *slog.Logger) {
	runJob := func() {
		count, err := job(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error(name+" failed", slog.String("error", err.Error()))
		} else if count > 0 {
			log.Info(name+" done", slog.Int64("count", count))
		}
	}
	runJob() // сразу при старте: при частых рестартах тик "раз в interval" иначе никогда не наступает
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			runJob()
		case <-ctx.Done():
			return
		}
	}
}

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load(log)

	pool := database.Connect(cfg.DBUrl, log)
	defer pool.Close()

	role := os.Getenv("ROLE")

	// Схема приложения — goose. Воркер её не накатывает: схему ведёт API-инстанс.
	if role != "worker" {
		if err := runMigrations(pool); err != nil {
			log.Error("goose migrate", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

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
	if role == "worker" {
		runWorker(pool, &cfg, log) // только воркер — выделенный ROLE=worker инстанс
		return
	}

	r, subscriptionService, wbHubManager := router.Setup(pool, log, &cfg)

	// Auto-complete: mark expired lessons as completed every minute
	lessonRepo := repository.NewLessonRepository(pool)
	pendingRepo := repository.NewPendingRegistrationRepository(pool)
	bgCtx, bgCancel := context.WithCancel(context.Background())
	var bgWg sync.WaitGroup
	bgWg.Go(func() {
		runIntervalLoop(bgCtx, 1*time.Minute, "auto-complete lessons", lessonRepo.AutoComplete, log)
	})
	bgWg.Go(func() {
		runIntervalLoop(bgCtx, 24*time.Hour, "subscription renew", subscriptionService.RenewDue, log)
	})
	bgWg.Go(func() {
		runIntervalLoop(bgCtx, 10*time.Minute, "cleanup pending registrations", pendingRepo.DeleteExpired, log)
	})
	bgWg.Go(func() {
		handlers.ListenBoardEvents(bgCtx, pool, wbHubManager, log)
	})
	// Запись доски: буфер уходит в БД раз в секунду, а на bgCancel дописывает
	// остаток. Поэтому bgWg.Wait() стоит ДО srv.Shutdown — иначе последняя
	// секунда рисования терялась бы на каждом деплое.
	bgWg.Go(func() {
		wbHubManager.Run(bgCtx)
	})
	bgWg.Go(func() {
		runIntervalLoop(bgCtx, 10*time.Minute, "board tombstone cleanup", wbHubManager.CleanupTombstones, log)
	})
	// ponytail: воркер PDF-импорта в том же процессе, что и API — одному
	// Railway-сервису отдельный процесс не нужен. Очередь River живёт в
	// Postgres, поэтому этот воркер безопасно сосуществует с любыми выделёнными
	// ROLE=worker инстансами. ROLE=api глушит его, когда рендер выносят с
	// API-боксов.
	if role != "api" {
		bgWg.Go(func() {
			if err := worker.Run(bgCtx, pool, &cfg, log); err != nil {
				log.Error("embedded worker", slog.String("error", err.Error()))
			}
		})
	}

	r.GET("/health", func(c *gin.Context) {
		if err := pool.Ping(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	srv := &http.Server{
		Addr:    cfg.ServerPort,
		Handler: r,
	}

	go func() {
		log.Info("Server is listening on", slog.String("port", cfg.ServerPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server failed", slog.String("error", err.Error()))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")
	bgCancel()
	bgWg.Wait()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("Server exited cleanly")
}

// runMigrations накатывает вкомпилированные goose-миграции поверх пула.
// ponytail: без advisory lock — сейчас схему накатывает единственный API-инстанс.
// При втором ROLE=api понадобится goose.WithLock, иначе два старта гонятся.
func runMigrations(pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}

// runWorker — роль worker: River-воркер PDF-импортов.
func runWorker(pool *pgxpool.Pool, cfg *config.Config, log *slog.Logger) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := worker.Run(ctx, pool, cfg, log); err != nil {
		log.Error("worker", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("worker exited cleanly")
}

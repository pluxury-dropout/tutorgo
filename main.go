package main

import (
	"context"
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
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

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

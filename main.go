package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"tutorgo/config"
	"tutorgo/database"
	"tutorgo/logger"
	"tutorgo/repository"
	"tutorgo/router"

	"github.com/gin-gonic/gin"
)

func main() {
	log := logger.New()
	cfg := config.Load()

	pool := database.Connect(cfg.DBUrl, log)
	defer pool.Close()

	r := router.Setup(pool, log, &cfg)

	// Auto-complete: mark expired lessons as completed every minute
	lessonRepo := repository.NewLessonRepository(pool)
	bgCtx, bgCancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				count, err := lessonRepo.AutoComplete(bgCtx)
				if err != nil {
					log.Error("Auto-complete failed", slog.String("error", err.Error()))
				} else if count > 0 {
					log.Info("Auto-completed lessons", slog.Int64("count", count))
				}
			case <-bgCtx.Done():
				return
			}
		}
	}()

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("Server exited cleanly")
}

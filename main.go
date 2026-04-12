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
	"tutorgo/router"

	"github.com/gin-gonic/gin"
)

func main() {
	log := logger.New()
	cfg := config.Load()

	pool := database.Connect(cfg.DBUrl, log)
	defer pool.Close()

	r := router.Setup(pool, log, &cfg)

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("Server exited cleanly")
}

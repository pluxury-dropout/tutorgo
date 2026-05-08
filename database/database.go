package database

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(dbURL string, log *slog.Logger) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Error("Failed to parse db config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Error("Failed to connect to db", slog.String("error", err.Error()))
		os.Exit(1)
	}

	if err := pool.Ping(context.Background()); err != nil {
		log.Error("Failed to ping db", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("Connected to database successfully")
	return pool
}

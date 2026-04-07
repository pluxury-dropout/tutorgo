package database

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(dbURL string, log *slog.Logger) *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), dbURL)
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

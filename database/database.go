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

	cfg.MinConns = 2
	cfg.MaxConns = 7
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Error("Failed to connect to db", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// База может быть ещё не готова: локально Postgres стартует параллельно с
	// приложением, в проде — переживает рестарт. Раньше этого ждал nc-луп в
	// entrypoint.sh, но он проверял открытый порт, а Postgres открывает его
	// до конца recovery. Ping — честная проверка.
	if err := waitReady(pool, log); err != nil {
		log.Error("Failed to ping db", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("Connected to database successfully")
	return pool
}

// waitReady пингует базу, пока та не ответит. Возвращает последнюю ошибку,
// если база так и не поднялась.
// ponytail: фиксированный интервал, не backoff — ждём максимум полминуты,
// экспоненте тут негде разогнаться. Сдаёмся, а не ждём вечно: упавший
// контейнер платформа перезапустит, зависший — примет за живой.
func waitReady(pool *pgxpool.Pool, log *slog.Logger) error {
	var err error
	for i := range 30 {
		if err = pool.Ping(context.Background()); err == nil {
			return nil
		}
		if i == 0 {
			log.Info("Waiting for database...")
		}
		time.Sleep(time.Second)
	}
	return err
}

package database

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect поднимает пул. Режим пулера Supabase выбирается ПОРТОМ в DB_URL, а не
// настройкой в дашборде — один и тот же Supavisor слушает оба:
//
//	:5432 — session mode. Реальное соединение к Postgres закреплено за клиентом
//	        до отключения. Работает всё, включая LISTEN/NOTIFY и advisory locks,
//	        но лимит соединений на проект низкий. Нужен ROLE=worker: River ловит
//	        задачи через LISTEN.
//	:6543 — transaction mode. Соединение выдаётся на время транзакции и сразу
//	        возвращается, поэтому клиентов помещается кратно больше. Session-level
//	        состояние (LISTEN, advisory locks, SET) не переживает границу
//	        транзакции. Годится ROLE=api: там таких потребителей не осталось —
//	        board-events уехали в Redis (см. pubsub), River-клиент insert-only.
//
// К :6543 строка подключения обязана нести ?default_query_exec_mode=exec, иначе
// pgx кеширует prepared statements на соединении, которое под ним меняется, и
// сыплет "prepared statement already exists". Параметр разбирает pgx.ParseConfig.
func Connect(dbURL string, maxConns int32, log *slog.Logger) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Error("Failed to parse db config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	cfg.MinConns = 2
	cfg.MaxConns = maxConns
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

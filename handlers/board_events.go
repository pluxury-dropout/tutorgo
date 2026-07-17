package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"tutorgo/models"
)

// ListenBoardEvents держит выделенное соединение с LISTEN board_events и
// пересылает события воркера в WS-хабы. Блокируется до отмены ctx; при обрыве
// соединения переподключается с паузой.
func ListenBoardEvents(ctx context.Context, pool *pgxpool.Pool, mgr *WbHubManager, log *slog.Logger) {
	for ctx.Err() == nil {
		if err := listenOnce(ctx, pool, mgr); err != nil && ctx.Err() == nil {
			log.Warn("board events listener", slog.String("error", err.Error()))
			time.Sleep(3 * time.Second)
		}
	}
}

func listenOnce(ctx context.Context, pool *pgxpool.Pool, mgr *WbHubManager) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	// Соединение испорчено LISTEN-состоянием — в пул его не возвращаем.
	defer conn.Conn().Close(context.Background()) //nolint:errcheck
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN board_events"); err != nil {
		return err
	}
	for {
		n, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var ev models.BoardEvent
		if err := json.Unmarshal([]byte(n.Payload), &ev); err != nil || ev.PageID == "" {
			continue // мусор в канале — не наш, пропускаем
		}
		mgr.PushToPage(ev.PageID, ev.Msg)
	}
}

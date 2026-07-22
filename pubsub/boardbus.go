// Package pubsub переносит события доски между процессами: PDF-воркер
// сообщает API-инстансам, что страница отрендерилась и её пора показать.
//
// Раньше это ехало через pg_notify/LISTEN. Postgres в этой роли стоил дорого:
// LISTEN требует персонального соединения, а их у Supabase считаное число.
// Redis той же цены не имеет — плюс payload не упирается в лимит 8000 байт,
// который у NOTIFY есть.
package pubsub

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"tutorgo/models"
)

const boardChannel = "board_events"

// localBuffer — сколько событий переживёт медленного подписчика в in-process
// режиме. Один PDF шлёт событие на страницу; сотня страниц в буфер влезает.
const localBuffer = 128

// BoardBus доставляет события доски. С Redis — между процессами, без него —
// внутри одного (dev: API и воркер живут в общем процессе).
//
// Доставка at-most-once в обоих режимах, и это осознанно: событие лишь ускоряет
// появление страницы, источник истины — files-карта в БД. Потерялось — юзер
// увидит страницу после reload (см. worker/pdf_import.go, "best-effort").
type BoardBus struct {
	rdb   *redis.Client
	local chan models.BoardEvent
	log   *slog.Logger
}

// New возвращает шину. Пустой redisURL — не ошибка, а in-process режим.
func New(redisURL string, log *slog.Logger) (*BoardBus, error) {
	if redisURL == "" {
		log.Info("board events: in-process bus (REDIS_URL not set)")
		return &BoardBus{local: make(chan models.BoardEvent, localBuffer), log: log}, nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	log.Info("board events: redis bus", slog.String("addr", opt.Addr))
	return &BoardBus{rdb: redis.NewClient(opt), log: log}, nil
}

// Publish отправляет событие подписчикам. Ошибку возвращает, но вызывающий
// волен её проглотить — доставка best-effort by design.
func (b *BoardBus) Publish(ctx context.Context, ev models.BoardEvent) error {
	if b.rdb == nil {
		select {
		case b.local <- ev:
		default:
			// Отправитель — горутина River-воркера: заблокируй её, и встанет
			// рендер PDF целиком. Событие дешевле рендера, поэтому теряем его,
			// но громко: молчаливая потеря потом не расследуется.
			b.log.Warn("board event bus full, drop event", slog.String("pageId", ev.PageID))
		}
		return nil
	}
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, boardChannel, payload).Err()
}

// Subscribe блокируется до отмены ctx и зовёт handle на каждое событие.
// Переподключение при обрыве Redis берёт на себя go-redis — своего цикла
// reconnect, в отличие от pgx-версии на LISTEN, здесь не нужно.
func (b *BoardBus) Subscribe(ctx context.Context, handle func(models.BoardEvent)) {
	if b.rdb == nil {
		for {
			select {
			case ev := <-b.local:
				handle(ev)
			case <-ctx.Done():
				return
			}
		}
	}

	sub := b.rdb.Subscribe(ctx, boardChannel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev models.BoardEvent
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil || ev.PageID == "" {
				continue // мусор в канале — не наш, пропускаем
			}
			handle(ev)
		case <-ctx.Done():
			return
		}
	}
}

// Close отпускает соединение с Redis. In-process режим закрывать нечего.
func (b *BoardBus) Close() error {
	if b.rdb == nil {
		return nil
	}
	return b.rdb.Close()
}

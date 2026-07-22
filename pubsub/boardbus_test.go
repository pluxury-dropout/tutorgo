package pubsub

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"tutorgo/models"
)

func testBus(t *testing.T) *BoardBus {
	t.Helper()
	bus, err := New("", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return bus
}

// In-process режим должен доставлять событие подписчику того же процесса —
// это единственный путь для dev-запуска, где REDIS_URL не задан.
func TestInProcessDelivers(t *testing.T) {
	bus := testBus(t)
	ctx := t.Context() // отменяется по завершении теста — Subscribe не утечёт

	got := make(chan models.BoardEvent, 1)
	go bus.Subscribe(ctx, func(ev models.BoardEvent) { got <- ev })

	want := models.BoardEvent{PageID: "page-1", Msg: []byte(`{"type":"file"}`)}
	if err := bus.Publish(ctx, want); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case ev := <-got:
		if ev.PageID != want.PageID {
			t.Errorf("PageID = %q, want %q", ev.PageID, want.PageID)
		}
		if string(ev.Msg) != string(want.Msg) {
			t.Errorf("Msg = %s, want %s", ev.Msg, want.Msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("событие не дошло до подписчика")
	}
}

// Publish не имеет права заблокировать вызывающего, даже когда подписчика нет
// и буфер переполнен: это горутина River-воркера, и её остановка подвесила бы
// рендер PDF целиком. Доставка событий — best-effort, рендер — нет.
func TestInProcessPublishNeverBlocks(t *testing.T) {
	bus := testBus(t)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for range localBuffer * 2 {
			// Ошибку игнорируем осознанно: проверяем только, что вызов вернулся.
			_ = bus.Publish(context.Background(), models.BoardEvent{PageID: "page-1"})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish заблокировался без подписчика — буфер конечен, отправка должна сдаваться")
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"tutorgo/models"
)

// sinkStub записывает, что буфер отдал на запись, и умеет падать по требованию.
type sinkStub struct {
	mu    sync.Mutex
	calls [][]models.BoardElement
	err   error
	done  chan struct{}
}

func (s *sinkStub) MergeElements(_ context.Context, _ string, els []models.BoardElement) error {
	s.mu.Lock()
	s.calls = append(s.calls, els)
	err := s.err
	s.mu.Unlock()
	if s.done != nil {
		select {
		case s.done <- struct{}{}:
		default:
		}
	}
	return err
}

func (s *sinkStub) lastCall() []models.BoardElement {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.calls) == 0 {
		return nil
	}
	return s.calls[len(s.calls)-1]
}

func newTestBuffer(sink *sinkStub) *elementBuffer {
	return newElementBuffer(sink, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func rawEl(id string, version int, nonce int64) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"id": id, "version": version, "versionNonce": nonce})
	return b
}

func payload(els ...json.RawMessage) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"elements": els})
	return b
}

// Схлопывание правок одного элемента идёт по тому же правилу, что и запись в БД.
// Иначе апдейт, пришедший позже, но с меньшей версией (переупорядоченная
// доставка, эхо второго инстанса), вытеснил бы из буфера свежую правку — и та
// не доехала бы до Postgres никогда.
func TestBufferKeepsWinner(t *testing.T) {
	sink := &sinkStub{}
	b := newTestBuffer(sink)

	b.addPayload("p1", payload(rawEl("a", 5, 100)))
	b.addPayload("p1", payload(rawEl("a", 3, 100))) // старая — не должна победить
	b.flush(context.Background())

	got := sink.lastCall()
	if len(got) != 1 {
		t.Fatalf("записано %d элементов, ожидался 1", len(got))
	}
	if got[0].Version != 5 {
		t.Fatalf("в БД уехала version=%d, ожидалась 5", got[0].Version)
	}
}

// Провал записи не должен стоить нарисованного: элементы возвращаются в буфер и
// уезжают следующим тиком.
func TestBufferRetriesAfterFailure(t *testing.T) {
	sink := &sinkStub{err: errors.New("db down")}
	b := newTestBuffer(sink)

	b.addPayload("p1", payload(rawEl("a", 1, 10)))
	b.flush(context.Background())

	sink.mu.Lock()
	sink.err = nil
	sink.mu.Unlock()
	b.flush(context.Background())

	if len(sink.calls) != 2 {
		t.Fatalf("попыток записи %d, ожидалось 2", len(sink.calls))
	}
	if got := sink.lastCall(); len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("элемент потерян при ретрае: %+v", got)
	}
}

// Успешная запись очищает буфер: без этого каждый тик переписывал бы всю доску
// заново — ровно та амплификация, ради ухода от которой всё и затевалось.
func TestBufferDrainsOnSuccess(t *testing.T) {
	sink := &sinkStub{}
	b := newTestBuffer(sink)

	b.addPayload("p1", payload(rawEl("a", 1, 10)))
	b.flush(context.Background())
	b.flush(context.Background())

	if len(sink.calls) != 1 {
		t.Fatalf("записей %d, ожидалась 1 — буфер не опустошился", len(sink.calls))
	}
}

// Штатный деплой не должен откусывать последнюю секунду рисования: на отмену
// контекста Run дописывает остаток по свежему контексту.
func TestBufferFlushesOnShutdown(t *testing.T) {
	sink := &sinkStub{done: make(chan struct{}, 1)}
	b := newTestBuffer(sink)
	ctx, cancel := context.WithCancel(context.Background())

	go b.Run(ctx)
	b.addPayload("p1", payload(rawEl("a", 1, 10)))
	cancel()

	select {
	case <-sink.done:
	case <-time.After(2 * time.Second):
		t.Fatal("буфер не дописан при завершении")
	}
	if got := sink.lastCall(); len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("дописано не то: %+v", got)
	}
}

// Мусор от клиента отбрасывается, но соседние валидные элементы — нет.
func TestBufferSkipsInvalidElements(t *testing.T) {
	sink := &sinkStub{}
	b := newTestBuffer(sink)

	b.addPayload("p1", payload(
		json.RawMessage(`{"version":1,"versionNonce":1}`), // без id
		rawEl("ok", 1, 10),
	))
	b.flush(context.Background())

	got := sink.lastCall()
	if len(got) != 1 || got[0].ID != "ok" {
		t.Fatalf("валидный элемент не пережил соседа-мусор: %+v", got)
	}
}

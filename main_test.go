package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunIntervalLoop_CallsJobThenStopsOnCancel(t *testing.T) {
	var calls atomic.Int64
	job := func(context.Context) (int64, error) {
		calls.Add(1)
		return 1, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	done := make(chan struct{})
	go func() {
		runIntervalLoop(ctx, 5*time.Millisecond, "test", job, log)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop не завершился после cancel")
	}
	assert.GreaterOrEqual(t, calls.Load(), int64(1))
	_ = errors.Is // держим импорт, если понадобится
}

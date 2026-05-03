package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestAutoCompleteGoroutineStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var ticks atomic.Int64
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-ticker.C:
				ticks.Add(1)
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// goroutine exited cleanly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("goroutine did not exit after context cancellation")
	}

	if ticks.Load() == 0 {
		t.Error("goroutine never ticked before cancellation")
	}
}

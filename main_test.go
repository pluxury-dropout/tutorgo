package main

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRunAutoCompleteLoop_ExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	called := make(chan struct{}, 10)
	stub := func(ctx context.Context) (int64, error) {
		called <- struct{}{}
		return 1, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		runAutoCompleteLoop(ctx, 10*time.Millisecond, stub, slog.Default())
	}()

	// wait for at least one call
	select {
	case <-called:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("autoComplete was never called")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runAutoCompleteLoop did not exit after context cancellation")
	}
}

func TestRunAutoCompleteLoop_ExitsImmediatelyIfContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	calls := 0
	stub := func(ctx context.Context) (int64, error) {
		calls++
		return 0, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		runAutoCompleteLoop(ctx, 1*time.Hour, stub, slog.Default())
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runAutoCompleteLoop did not exit with already-cancelled context")
	}
}

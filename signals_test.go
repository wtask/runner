package daemon

import (
	"context"
	"syscall"
	"testing"
	"time"
)

func Test_waitSignal(t *testing.T) {
	// avoid running sub tests in parallel
	t.Run("Signal", waitSignal_signal)
	t.Run("Context", waitsSignal_context)
}

func waitSignal_signal(t *testing.T) {

	done := make(chan struct{})
	go func() {
		defer close(done)

		waitSignal(context.Background())
	}()

	go func() {
		time.Sleep(250 * time.Millisecond)
		signals <- syscall.SIGTERM
	}()

	timeout := 5 * time.Second
	select {
	case <-done:
		if len(signals) != 0 {
			t.Fatal("signals not cleaned")
		}
	case <-time.After(timeout):
		t.Fatalf("timeout expired: not done in %s", timeout)
	}
}

func waitsSignal_context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)

		waitSignal(ctx)
	}()

	go func() {
		time.Sleep(250 * time.Millisecond)
		cancel()
	}()

	timeout := 5 * time.Second
	select {
	case <-done:
		if len(signals) != 0 {
			t.Fatal("signals not cleaned")
		}
	case <-time.After(timeout):
		t.Fatalf("timeout expired: not done in %s", timeout)
	}
}

package daemon

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	// avoid run sub tests in parallel
	t.Run("Context", run_context)
	t.Run("Signal", run_signal)
	t.Run("Service-Error", run_service_error)
	t.Run("Service-Panic", run_service_panic)
}

func run_context(t *testing.T) {
	if IsStarted() {
		t.Fatal("started unexpectedly")
	}

	done := struct {
		run                chan error
		service1, service2 bool
	}{
		run: make(chan error, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	timeout := 5 * time.Second

	go func() {
		defer close(done.run)

		done.run <- Run(
			ctx,
			WithService(func(ctx context.Context) error {
				select {
				case <-time.After(timeout):
					return fmt.Errorf("service 1: timeout expired")
				case <-ctx.Done():
					done.service1 = true
					return nil
				}
			}),
			WithService(func(ctx context.Context) error {
				select {
				case <-time.After(timeout):
					return fmt.Errorf("service 2: timeout expired")
				case <-ctx.Done():
					done.service2 = true
					return nil
				}
			}),
		)
	}()

	go func() {
		time.Sleep(250 * time.Millisecond)
		cancel()
	}()

	time.Sleep(50 * time.Millisecond)

	if !IsStarted() {
		t.Fatal("not started")
	}

	select {
	case err := <-done.run:
		switch {
		case err != nil:
			t.Fatal(err)
		case !done.service1:
			t.Fatal("service 1: not done")
		case !done.service2:
			t.Fatal("service 2: not done")
		case IsStarted():
			t.Fatal("daemon is still running")
		}
	case <-time.After(timeout):
		t.Fatalf("timeout expired: not done in %s", timeout)
	}
}

func run_signal(t *testing.T) {
	if IsStarted() {
		t.Fatal("started unexpectedly")
	}

	done := struct {
		run                chan error
		service1, service2 bool
	}{
		run: make(chan error, 1),
	}
	timeout := 5 * time.Second

	go func() {
		defer close(done.run)

		done.run <- Run(
			context.Background(),
			WithService(func(ctx context.Context) error {
				select {
				case <-time.After(timeout):
					return fmt.Errorf("service 1: timeout expired")
				case <-ctx.Done():
					done.service1 = true
					return nil
				}
			}),
			WithService(func(ctx context.Context) error {
				select {
				case <-time.After(timeout):
					return fmt.Errorf("service 2: timeout expired")
				case <-ctx.Done():
					done.service2 = true
					return nil
				}
			}),
		)
	}()

	go func() {
		time.Sleep(250 * time.Millisecond)
		signals <- syscall.SIGINT
	}()

	time.Sleep(50 * time.Millisecond)

	if !IsStarted() {
		t.Fatal("not started")
	}

	select {
	case err := <-done.run:
		switch {
		case err != nil:
			t.Fatal(err)
		case !done.service1:
			t.Fatal("service 1: not done")
		case !done.service2:
			t.Fatal("service 2: not done")
		case IsStarted():
			t.Fatal("daemon is still running")
		}
	case <-time.After(timeout):
		t.Fatalf("timeout expired: not done in %s", timeout)
	}
}

func run_service_error(t *testing.T) {
	done := struct {
		run                chan error
		service1, service2 bool
	}{
		run: make(chan error, 1),
	}

	errFailure := errors.New("failure")

	go func() {
		defer close(done.run)

		done.run <- Run(
			context.Background(),
			WithService(func(ctx context.Context) error {
				time.Sleep(250 * time.Millisecond)
				return fmt.Errorf("service 1: %w", errFailure)
			}),
			WithService(func(ctx context.Context) error {
				<-ctx.Done()
				done.service2 = true
				return nil
			}),
		)
	}()

	timeout := 5 * time.Second
	select {
	case err := <-done.run:
		switch {
		case !errors.Is(err, errFailure):
			t.Fatalf("unexpected error: %s", err)
		case !done.service2:
			t.Fatal("service 2: not done")
		case IsStarted():
			t.Fatal("daemon is still running")
		}
	case <-time.After(timeout):
		t.Fatalf("timeout expired: not done in %s", timeout)
	}
}

func run_service_panic(t *testing.T) {
	done := struct {
		run                chan error
		service1, service2 bool
	}{
		run: make(chan error, 1),
	}

	go func() {
		defer close(done.run)

		done.run <- Run(
			context.Background(),
			WithService(func(ctx context.Context) error {
				<-ctx.Done()
				done.service1 = true
				return nil
			}),
			WithService(func(ctx context.Context) error {
				time.Sleep(250 * time.Millisecond)
				panic("service 2 panic")
			}),
		)
	}()

	timeout := 5 * time.Second
	select {
	case err := <-done.run:
		switch {
		case err == nil:
			t.Fatalf("error expected got nil")
		case !done.service1:
			t.Fatal("service 1: not done")
		case IsStarted():
			t.Fatal("daemon is still running")
		}
	case <-time.After(timeout):
		t.Fatalf("timeout expired: not done in %s", timeout)
	}
}

package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

var started int32

func IsStarted() bool {
	return atomic.CompareAndSwapInt32(&started, 1, 1)
}

// Run starts required goroutines for services are specified through the options
// and then blocks execution until termination signal will received from OS or any service return an error.
func Run(ctx context.Context, options ...Option) error {
	if !atomic.CompareAndSwapInt32(&started, 0, 1) {
		return fmt.Errorf("already started")
	}

	defer func() {
		resetOptions()
		atomic.CompareAndSwapInt32(&started, 1, 0)
	}()

	if err := applyOptions(options...); err != nil {
		return err
	}

	c, cancel := context.WithCancel(ctx)
	threads, background := errgroup.WithContext(c)

	for _, service := range services {
		if service == nil {
			continue
		}

		service := service
		threads.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("service thread failure: %v", r)
				}
			}()

			return service(background)
		})
	}

	waitSignal(background)
	cancel()

	return release(threads.Wait)
}

func waitSignal(ctx context.Context) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals) // all available signals

	defer signal.Stop(signals)

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-signals:
			if _, ok := stops[sig]; ok {
				return
			}

			if action, ok := continueActions[sig]; ok && action != nil {
				go action(ctx)
			}
		}
	}
}

func release(wait func() error) error {
	done := make(chan error, 1)
	go func() {
		defer close(done)

		done <- wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(standstill):
		return fmt.Errorf("standstill period (%s) expired: services were not stopped in time", standstill)
	}
}

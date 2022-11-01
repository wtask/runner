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

// IsStarted returns true when daemon.Run() is started and false otherwise.
func IsStarted() bool {
	return atomic.CompareAndSwapInt32(&started, 1, 1)
}

// Run starts required goroutines for jobs are specified through the options
// and then blocks execution until termination signal will received from OS
// or any job will return an error or all jobs will done.
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

	ctx, cancel := context.WithCancel(ctx)
	runner, background := errgroup.WithContext(ctx)

	for _, job := range jobs {
		if job == nil {
			continue
		}

		job := job
		runner.Go(func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("job failure: %v", r)
				}
			}()

			return job(background)
		})
	}

	waitSignal(background)
	cancel()

	return release(standstill, runner.Wait)
}

var signals = make(chan os.Signal, 1)

func waitSignal(ctx context.Context) {
	signal.Notify(signals) // all available os signals

	defer func() {
		signal.Stop(signals)
		signal.Reset()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-signals:
			if _, ok := stops[sig]; ok {
				return
			}

			if action := actions[sig]; action != nil {
				go action(ctx)
			}
		}
	}
}

// release runs wait-function in separate goroutine and returns its result
// or error after specified time is expired.
func release(after time.Duration, wait func() error) error {
	done := make(chan error, 1)
	go func() {
		defer close(done)

		done <- wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(after):
		return fmt.Errorf("forced release: awaiting period (%s) has expired", after)
	}
}

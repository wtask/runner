package daemon

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
)

// Option represents function that can be executed before the daemon starts.
type Option func() error

var (
	stops = map[os.Signal]struct{}{
		syscall.SIGINT:  {},
		syscall.SIGTERM: {},
	}

	continueActions = make(map[os.Signal]func(context.Context))

	services []func(context.Context) error

	standstill = 5 * time.Second // Period to wait for all services will stop before exit from Run
)

// WithStopSignal allows to append specified signal to daemon's default stop list.
// Note, SIGINT and SIGTERM are registered by default.
func WithStopSignal(sig os.Signal) Option {
	return func() error {
		stops[sig] = struct{}{}

		return nil
	}
}

// WithContinueAction allows to add/replace action for specified signal.
func WithContinueAction(sig os.Signal, action func(context.Context)) Option {
	return func() error {
		continueActions[sig] = action

		return nil
	}
}

// WithService allows to register specified service to start with daemon context.
// It is important the service will be able to immediately stop when context will done.
func WithService(service func(context.Context) error) Option {
	return func() error {
		services = append(services, service)

		return nil
	}
}

// WithStandstill allows to change default period (5 sec) is required to wait for running services to stop.
func WithStandstill(period time.Duration) Option {
	return func() error {
		if period < 0 {
			return fmt.Errorf("invalid standstill period: %s", period)
		}

		standstill = period

		return nil
	}
}

func applyOptions(options ...Option) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("applying options failure: %v", r)
		}
	}()

	for _, option := range options {
		if option == nil {
			continue
		}

		if err = option(); err != nil {
			return err
		}
	}

	return nil
}

func resetOptions() {
	for k := range stops {
		if !(k == syscall.SIGINT || k == syscall.SIGTERM) {
			delete(stops, k)
		}
	}

	for k := range continueActions {
		delete(continueActions, k)
	}

	services = services[:0]
	standstill = 5 * time.Second
}

package daemon

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
)

// Option represents an optional feature for the runner.
// Under the hood the option is a function which will be executed
// before the daemon blocks on listening the os signals.
// You are free to define custom options affected on your own code base.
type Option func() error

var (
	// stop signals
	stops = map[os.Signal]struct{}{
		syscall.SIGINT:  {},
		syscall.SIGTERM: {},
	}

	// signal actions
	actions = make(map[os.Signal]func(context.Context))

	jobs []func(context.Context) error

	// standstill is a period to wait for all jobs will stop before exit from daemon.Run()
	standstill = defaultStandstill
)

const (
	defaultStandstill = 5 * time.Second
)

// WithStopSignal allows to append specified signal to daemon's default stop list.
// Note, SIGINT and SIGTERM are registered by default.
// When stop signal is received the runner will cancel his jobs
// and will exit not later than the standstill time expires.
func WithStopSignal(sig os.Signal) Option {
	return func() error {
		stops[sig] = struct{}{}

		return nil
	}
}

// WithSignalAction allows to register an action for specified signal.
// When signal is received the corresponding action will run in separate goroutine
// but the runner will continue work.
// It is allowed to register any signal action with this option,
// however the stop signals are processed first, so their actions always ignored
// if they are registered here (SIGTERM and SIGINT by default).
func WithSignalAction(sig os.Signal, action func(context.Context)) Option {
	return func() error {
		actions[sig] = action

		return nil
	}
}

// WithJob allows to register specified function as background job running with daemon context.
// It is important that the job will be able to immediately stop when context will done.
// All jobs are cancelled when one of them returns with error
// or daemon runner received a stop signal.
func WithJob(job func(context.Context) error) Option {
	return func() error {
		jobs = append(jobs, job)

		return nil
	}
}

// WithStandstill allows to change default period (5 sec) which is required
// to wait for all running jobs to stop before the runner exits.
// If any of background jobs can not stop in this period then it is lost.
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

// resetOptions sets internal package vars to their default values
func resetOptions() {
	for k := range stops {
		if !(k == syscall.SIGINT || k == syscall.SIGTERM) {
			delete(stops, k)
		}
	}

	for k := range actions {
		delete(actions, k)
	}

	jobs = jobs[:0]
	standstill = defaultStandstill
}

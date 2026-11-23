// Package retry executes operations with an injected retry strategy.
package retry

import (
	"context"
	"errors"
	"time"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=retryer.go -destination=retryer_mock_test.go -package=retry_test

// Strategy classifies an error, limits attempts and calculates the next delay.
// failedAttempt counts the initial failed call as one. Implementations must be
// safe for concurrent use; attempt counters belong to Do, not to the strategy.
type Strategy interface {
	Next(failedAttempt int, err error) (time.Duration, bool)
}

// Retryer repeats a complete operation without owning its resources or results.
type Retryer struct{ strategy Strategy }

// NewRetryer receives a configured strategy shared by this retryer's callers.
func NewRetryer(strategy Strategy) *Retryer { return &Retryer{strategy: strategy} }

// Do returns nil on success or the last operation error when retries are refused
// or exhausted. Each attempt must release its resources and discard partial
// results before returning. Cancellation and PermanentError prohibit retries.
func (r *Retryer) Do(ctx context.Context, operation func() error) error {
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := operation()
		if err == nil {
			return nil
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		var permanent *PermanentError
		if errors.As(err, &permanent) {
			return err
		}

		delay, again := r.strategy.Next(attempt, err)
		if !again {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}

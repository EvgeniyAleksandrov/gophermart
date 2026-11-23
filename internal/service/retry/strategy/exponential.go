// Package strategy provides error classification and delay policies for retry.
package strategy

import (
	"errors"
	"time"
)

// Checker identifies errors permitted by an exponential retry strategy.
type Checker interface{ Matches(error) bool }

// Exponential bounds the number of attempts and doubles delays up to a limit.
type Exponential struct {
	maxAttempts            int
	initialDelay, maxDelay time.Duration
	checker                Checker
}

// NewExponential validates the attempt limit, nonnegative initial delay and
// positive delay cap. A zero initial delay permits immediate retries.
func NewExponential(maxAttempts int, initialDelay, maxDelay time.Duration, checker Checker) (*Exponential, error) {
	if maxAttempts < 1 || initialDelay < 0 || maxDelay <= 0 || checker == nil {
		return nil, errors.New("create exponential strategy: invalid configuration")
	}

	return &Exponential{maxAttempts: maxAttempts, initialDelay: initialDelay, maxDelay: maxDelay, checker: checker}, nil
}

// Next permits matching failures while the total attempt limit has not been met.
// Delay arithmetic saturates before multiplication to avoid duration overflow.
func (s *Exponential) Next(failedAttempt int, err error) (time.Duration, bool) {
	if err == nil || failedAttempt < 1 || failedAttempt >= s.maxAttempts || !s.checker.Matches(err) {
		return 0, false
	}

	delay := min(s.initialDelay, s.maxDelay)
	for n := 1; n < failedAttempt && delay > 0 && delay < s.maxDelay; n++ {
		if delay > s.maxDelay/2 {
			return s.maxDelay, true
		}
		delay *= 2
	}

	return delay, true
}

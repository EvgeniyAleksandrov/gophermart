// Package strategy provides error classification and delay policies for retry.
package strategy

import (
	"errors"
	"time"
)

type Checker interface {
	Matches(error) bool
}

type Exponential struct {
	maxAttempts  int
	initialDelay time.Duration
	maxDelay     time.Duration
	checker      Checker
}

func NewExponential(
	maxAttempts int,
	initialDelay, maxDelay time.Duration,
	checker Checker,
) (*Exponential, error) {
	if maxAttempts < 1 || initialDelay < 0 || maxDelay <= 0 || checker == nil {
		return nil, errors.New("create exponential strategy: invalid configuration")
	}

	return &Exponential{
		maxAttempts:  maxAttempts,
		initialDelay: initialDelay,
		maxDelay:     maxDelay,
		checker:      checker,
	}, nil
}

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

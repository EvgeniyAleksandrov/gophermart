package strategy

import (
	"errors"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

// RateLimit retries external rate limits using the server's validated delay.
// It has no attempt limit: the operation context bounds its lifetime.
type RateLimit struct{}

// NewRateLimit constructs a stateless rate-limit strategy.
func NewRateLimit() *RateLimit { return &RateLimit{} }

// Next permits only RateLimitError, preserving an explicitly zero delay.
func (s *RateLimit) Next(failedAttempt int, err error) (time.Duration, bool) {
	var limited *model.RateLimitError
	if failedAttempt < 1 || !errors.As(err, &limited) || limited.RetryAfter < 0 {
		return 0, false
	}

	return limited.RetryAfter, true
}

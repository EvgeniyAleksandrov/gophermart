package strategy

import (
	"errors"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type RateLimit struct {
}

func NewRateLimit() *RateLimit { return &RateLimit{} }

func (s *RateLimit) Next(failedAttempt int, err error) (time.Duration, bool) {
	var limited *model.RateLimitError
	if failedAttempt < 1 || !errors.As(err, &limited) || limited.RetryAfter < 0 {
		return 0, false
	}

	return limited.RetryAfter, true
}

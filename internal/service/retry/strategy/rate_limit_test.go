package strategy_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/stretchr/testify/require"
)

func TestRateLimit(t *testing.T) {
	for name, tc := range map[string]struct {
		err   error
		delay time.Duration
		again bool
	}{
		"server delay": {
			err:   fmt.Errorf("request: %w", &model.RateLimitError{RetryAfter: time.Minute}),
			delay: time.Minute,
			again: true,
		},
		"zero": {
			err:   &model.RateLimitError{},
			again: true,
		},
		"negative": {
			err: &model.RateLimitError{RetryAfter: -1},
		},
		"unavailable": {
			err: model.ErrAccrualUnavailable,
		},
		"not registered": {
			err: model.ErrAccrualNotRegistered,
		},
		"invalid response": {
			err: model.ErrAccrualInvalidResponse,
		},
		"nil": {},
	} {
		t.Run(name, func(t *testing.T) {
			delay, again := strategy.NewRateLimit().Next(10000, tc.err)
			require.Equal(t, tc.delay, delay)
			require.Equal(t, tc.again, again)
		})
	}
	_, again := strategy.NewRateLimit().Next(0, &model.RateLimitError{})
	require.False(t, again)
}

package strategy_test

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestExponentialNext(t *testing.T) {
	serial := &pgconn.PgError{Code: "40001"}
	policy, err := strategy.NewExponential(5, time.Second, 10*time.Second, strategy.NewSQLState([]string{"40001", "40P01"}))
	require.NoError(t, err)
	for name, tc := range map[string]struct {
		attempt int
		err     error
		delay   time.Duration
		again   bool
	}{
		"serialization": {
			attempt: 1,
			err:     fmt.Errorf("wrapped: %w", serial),
			delay:   time.Second,
			again:   true,
		},
		"deadlock": {
			attempt: 2,
			err:     &pgconn.PgError{Code: "40P01"},
			delay:   2 * time.Second,
			again:   true,
		},
		"third": {
			attempt: 3,
			err:     serial,
			delay:   4 * time.Second,
			again:   true,
		},
		"fourth": {
			attempt: 4,
			err:     serial,
			delay:   8 * time.Second,
			again:   true,
		},
		"exhausted": {
			attempt: 5,
			err:     serial,
		},
		"invalid attempt": {
			err: serial,
		},
		"constraint": {
			attempt: 1,
			err:     &pgconn.PgError{Code: "23505"},
		},
		"network": {
			attempt: 1,
			err:     errors.New("network failed"),
		},
		"nil": {
			attempt: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			delay, again := policy.Next(tc.attempt, tc.err)
			require.Equal(t, tc.again, again)
			require.Equal(t, tc.delay, delay)
		})
	}
}

func TestExponentialBounds(t *testing.T) {
	checker := strategy.NewSQLState([]string{"40001"})
	failure := &pgconn.PgError{Code: "40001"}
	for name, tc := range map[string]struct {
		initial time.Duration
		cap     time.Duration
		want    time.Duration
	}{
		"saturates": {
			initial: time.Second,
			cap:     10 * time.Second,
			want:    10 * time.Second,
		},
		"initial above cap": {
			initial: time.Minute,
			cap:     10 * time.Second,
			want:    10 * time.Second,
		},
		"overflow protected": {
			initial: time.Duration(math.MaxInt64/2 + 1),
			cap:     time.Duration(math.MaxInt64),
			want:    time.Duration(math.MaxInt64),
		},
		"zero delay": {
			cap: time.Second,
		},
	} {
		t.Run(name, func(t *testing.T) {
			policy, err := strategy.NewExponential(101, tc.initial, tc.cap, checker)
			require.NoError(t, err)
			delay, again := policy.Next(100, failure)
			require.True(t, again)
			require.Equal(t, tc.want, delay)
		})
	}
}

func TestExponentialConfiguration(t *testing.T) {
	for name, tc := range map[string]struct {
		attempts int
		initial  time.Duration
		cap      time.Duration
		checker  strategy.Checker
	}{
		"attempts": {
			cap:     time.Second,
			checker: strategy.NewSQLState(nil),
		},
		"negative initial": {
			attempts: 1,
			initial:  -1,
			cap:      time.Second,
			checker:  strategy.NewSQLState(nil),
		},
		"zero cap": {
			attempts: 1,
			checker:  strategy.NewSQLState(nil),
		},
		"nil checker": {
			attempts: 1,
			cap:      time.Second,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := strategy.NewExponential(tc.attempts, tc.initial, tc.cap, tc.checker)
			require.Error(t, err)
		})
	}
}

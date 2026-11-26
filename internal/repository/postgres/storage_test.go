package postgres_test

import (
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/stretchr/testify/require"
)

func singleAttempt(t *testing.T) *retry.Retryer { return retryPolicy(t, 1) }

func retryPolicy(t *testing.T, attempts int) *retry.Retryer {
	t.Helper()
	policy, err := strategy.NewExponential(
		attempts,
		0,
		time.Second,
		strategy.NewSQLState([]string{"40001", "40P01"}),
	)
	require.NoError(t, err)

	return retry.NewRetryer(policy)
}

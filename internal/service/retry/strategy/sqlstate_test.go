package strategy_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestSQLState(t *testing.T) {
	codes := []string{"40001"}
	checker := strategy.NewSQLState(codes)
	codes[0] = "23505"
	require.True(t, checker.Matches(fmt.Errorf("operation: %w", &pgconn.PgError{Code: "40001"})))
	require.False(t, checker.Matches(&pgconn.PgError{Code: "23505"}))
	require.False(t, checker.Matches(errors.New("SQLSTATE 40001")))
	require.False(t, checker.Matches(nil))
}

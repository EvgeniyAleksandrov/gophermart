package mapper_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/mapper"
	"github.com/stretchr/testify/require"
)

func TestWithdrawalPresent(t *testing.T) {
	stamp := time.Date(2026, 11, 23, 12, 0, 0, 0, time.FixedZone("MSK", 3*60*60))
	entries := []model.Withdrawal{
		{ID: 3, UserID: 7, OrderNumber: "18", Sum: math.MaxInt64, ProcessedAt: stamp},
		{ID: 2, UserID: 7, OrderNumber: "0", Sum: 1, ProcessedAt: stamp},
	}
	got, err := mapper.NewWithdrawal().Present(entries)
	require.NoError(t, err)
	body, err := json.Marshal(got)
	require.NoError(t, err)
	require.Equal(
		t,
		`[{"order":"18","sum":92233720368547758.07,"processed_at":"2026-11-23T09:00:00Z"},{"order":"0","sum":0.01,"processed_at":"2026-11-23T09:00:00Z"}]`,
		string(body),
	)
	entries[1].Sum = -1
	got, err = mapper.NewWithdrawal().Present(entries)
	require.Error(t, err)
	require.Nil(t, got)
	got, err = mapper.NewWithdrawal().Present(nil)
	require.NoError(t, err)
	require.Empty(t, got)
}

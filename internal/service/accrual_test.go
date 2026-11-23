package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces/mock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAccrualProcess(t *testing.T) {
	for name, tc := range map[string]struct {
		fetchErr error
		saveErr  error
	}{
		"success": {},
		"unknown order": {
			fetchErr: model.ErrAccrualNotRegistered,
		},
		"rate limited": {
			fetchErr: &model.RateLimitError{RetryAfter: time.Minute},
		},
		"invalid response": {
			fetchErr: model.ErrAccrualInvalidResponse,
		},
		"unavailable": {
			fetchErr: model.ErrAccrualUnavailable,
		},
		"save failure": {
			saveErr: errors.New("save failed"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage, client := mock.NewMockDBStorage(ctrl), NewMockAccrualClient(ctrl)
			ctx := context.Background()
			order := model.Order{ID: 1, UserID: 7, Number: "18"}
			result := model.AccrualResult{Number: "18", Status: model.AccrualProcessed, Accrual: 123}
			client.EXPECT().Fetch(ctx, "18").Return(result, tc.fetchErr)
			if tc.fetchErr == nil {
				storage.EXPECT().ApplyAccrual(ctx, int64(1), int64(7), result).Return(tc.saveErr)
			}

			s := service.NewAccrual(storage, client)
			err := s.Process(ctx, order)
			switch {
			case tc.fetchErr != nil:
				require.ErrorIs(t, err, tc.fetchErr)
			case tc.saveErr != nil:
				var persistence *model.PersistenceError
				require.ErrorAs(t, err, &persistence)
				require.ErrorIs(t, err, tc.saveErr)
			default:
				require.NoError(t, err)
			}
			storage.EXPECT().ListPending(ctx).Return([]model.Order{order}, nil)
			orders, err := s.Pending(ctx)
			require.NoError(t, err)
			require.Equal(t, []model.Order{order}, orders)
		})
	}
}

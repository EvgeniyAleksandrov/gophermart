package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces/mock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestBalanceGet(t *testing.T) {
	failure := errors.New("database failed")
	for name, tc := range map[string]struct {
		userID     int64
		exists     bool
		existsErr  error
		balanceErr error
		deleted    bool
		recheckErr error
		wantErr    error
	}{
		"success": {
			userID: 7,
			exists: true,
		},
		"invalid ID": {
			wantErr: model.ErrUserNotFound,
		},
		"deleted": {
			userID:  7,
			wantErr: model.ErrUserNotFound,
		},
		"existence failure": {
			userID:    7,
			existsErr: failure,
			wantErr:   failure,
		},
		"balance failure": {
			userID:     7,
			exists:     true,
			balanceErr: failure,
			wantErr:    failure,
		},
		"deleted between queries": {
			userID:     7,
			exists:     true,
			balanceErr: model.ErrBalanceNotFound,
			deleted:    true,
			wantErr:    model.ErrUserNotFound,
		},
		"missing mandatory balance": {
			userID:     7,
			exists:     true,
			balanceErr: model.ErrBalanceNotFound,
			wantErr:    model.ErrBalanceNotFound,
		},
		"recheck failed": {
			userID:     7,
			exists:     true,
			balanceErr: model.ErrBalanceNotFound,
			recheckErr: failure,
			wantErr:    failure,
		},
	} {
		t.Run(name, func(t *testing.T) {
			storage := mock.NewMockDBStorage(gomock.NewController(t))
			ctx := context.Background()
			balance := model.Balance{Current: 123, Withdrawn: 456}
			var calls []any
			if tc.userID > 0 {
				calls = append(calls, storage.EXPECT().UserExists(ctx, tc.userID).Return(tc.exists, tc.existsErr))
			}

			if tc.exists && tc.existsErr == nil {
				calls = append(calls, storage.EXPECT().GetBalance(ctx, tc.userID).Return(balance, tc.balanceErr))
				if tc.balanceErr == model.ErrBalanceNotFound {
					calls = append(calls, storage.EXPECT().UserExists(ctx, tc.userID).Return(!tc.deleted, tc.recheckErr))
				}
			}
			gomock.InOrder(calls...)
			got, err := service.NewBalance(storage).Get(ctx, tc.userID)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Zero(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, balance, got)
		})
	}
}

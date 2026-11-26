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

func TestWithdrawalWithdraw(t *testing.T) {
	failure := errors.New("storage failed")

	for name, tc := range map[string]struct {
		sum         int64
		number      string
		userID      int64
		exists      bool
		existsErr   error
		withdrawErr error
		wantErr     error
	}{
		"success": {
			sum:    1234,
			number: "18",
			userID: 7,
			exists: true,
		},
		"zero number": {
			sum:    1,
			number: "0",
			userID: 7,
			exists: true,
		},
		"sum before number and user": {
			wantErr: model.ErrInvalidWithdrawalAmount,
		},
		"negative sum": {
			sum:     -1,
			wantErr: model.ErrInvalidWithdrawalAmount,
		},
		"number before user": {
			sum:     1,
			wantErr: model.ErrInvalidOrderNumber,
		},
		"external spaces": {
			sum:     1,
			number:  " 18 ",
			userID:  7,
			wantErr: model.ErrInvalidOrderNumber,
		},
		"internal spaces": {
			sum:     1,
			number:  "1 8",
			userID:  7,
			wantErr: model.ErrInvalidOrderNumber,
		},
		"leading zero": {
			sum:     1,
			number:  "018",
			userID:  7,
			wantErr: model.ErrInvalidOrderNumber,
		},
		"non ASCII": {
			sum:     1,
			number:  "１８",
			userID:  7,
			wantErr: model.ErrInvalidOrderNumber,
		},
		"bad checksum": {
			sum:     1,
			number:  "19",
			userID:  7,
			wantErr: model.ErrInvalidOrderNumber,
		},
		"zero ID": {
			sum:     1,
			number:  "18",
			wantErr: model.ErrUserNotFound,
		},
		"negative ID": {
			sum:     1,
			number:  "18",
			userID:  -1,
			wantErr: model.ErrUserNotFound,
		},
		"deleted user": {
			sum:     1,
			number:  "18",
			userID:  7,
			wantErr: model.ErrUserNotFound,
		},
		"existence failure": {
			sum:       1,
			number:    "18",
			userID:    7,
			existsErr: failure,
			wantErr:   failure,
		},
		"insufficient": {
			sum:         1234,
			number:      "18",
			userID:      7,
			exists:      true,
			withdrawErr: model.ErrInsufficientFunds,
			wantErr:     model.ErrInsufficientFunds,
		},
		"conflict": {
			sum:         1234,
			number:      "18",
			userID:      7,
			exists:      true,
			withdrawErr: model.ErrWithdrawalConflict,
			wantErr:     model.ErrWithdrawalConflict,
		},
		"storage failure": {
			sum:         1234,
			number:      "18",
			userID:      7,
			exists:      true,
			withdrawErr: failure,
			wantErr:     failure,
		},
	} {
		t.Run(name, func(t *testing.T) {
			storage := mock.NewMockDBStorage(gomock.NewController(t))
			validator := service.NewOrderNumberValidator()
			ctx := context.Background()
			if tc.sum > 0 && validator.Validate(tc.number) == nil && tc.userID > 0 {
				check := storage.EXPECT().UserExists(ctx, tc.userID).Return(tc.exists, tc.existsErr)
				if tc.exists && tc.existsErr == nil {
					storage.EXPECT().Withdraw(ctx, tc.userID, tc.number, tc.sum).Return(tc.withdrawErr).
						After(check)
				}
			}

			err := service.NewWithdrawal(storage, validator).Withdraw(ctx, tc.userID, tc.number, tc.sum)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestWithdrawalList(t *testing.T) {
	failure := errors.New("storage failed")

	for name, tc := range map[string]struct {
		userID    int64
		exists    bool
		existsErr error
		rows      []model.Withdrawal
		listErr   error
		wantErr   error
	}{
		"history": {
			userID: 7,
			exists: true,
			rows:   []model.Withdrawal{{ID: 9, UserID: 7, OrderNumber: "18", Sum: 100}},
		},
		"empty": {
			userID: 7,
			exists: true,
		},
		"zero ID": {
			wantErr: model.ErrUserNotFound,
		},
		"negative ID": {
			userID:  -1,
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
		"list failure discards result": {
			userID:  7,
			exists:  true,
			rows:    []model.Withdrawal{{ID: 9}},
			listErr: failure,
			wantErr: failure,
		},
	} {
		t.Run(name, func(t *testing.T) {
			storage := mock.NewMockDBStorage(gomock.NewController(t))
			ctx := context.Background()
			if tc.userID > 0 {
				check := storage.EXPECT().UserExists(ctx, tc.userID).Return(tc.exists, tc.existsErr)
				if tc.exists && tc.existsErr == nil {
					storage.EXPECT().ListWithdrawalsByUser(ctx, tc.userID).Return(tc.rows, tc.listErr).
						After(check)
				}
			}

			got, err := service.NewWithdrawal(storage, service.NewOrderNumberValidator()).List(ctx, tc.userID)
			require.ErrorIs(t, err, tc.wantErr)
			if tc.wantErr != nil {
				require.Nil(t, got)
			} else {
				require.Equal(t, tc.rows, got)
			}
		})
	}
}

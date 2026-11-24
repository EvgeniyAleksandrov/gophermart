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

func TestOrderUpload(t *testing.T) {
	storageErr := errors.New("storage failure")
	tests := map[string]struct {
		userID        int64
		raw           string
		number        string
		validationErr error
		exists        bool
		existsErr     error
		created       bool
		addErr        error
		wantErr       error
	}{
		"new trimmed": {
			userID:  7,
			raw:     " \t18\r\n",
			number:  "18",
			exists:  true,
			created: true,
		},
		"repeat": {
			userID: 7,
			raw:    "18",
			number: "18",
			exists: true,
		},
		"invalid": {
			userID:        7,
			raw:           " 1 8 ",
			number:        "1 8",
			validationErr: model.ErrInvalidOrderNumber,
			wantErr:       model.ErrInvalidOrderNumber,
		},
		"empty": {
			userID:        7,
			raw:           " \n",
			validationErr: model.ErrEmptyOrderNumber,
			wantErr:       model.ErrEmptyOrderNumber,
		},
		"zero user": {
			wantErr: model.ErrUserNotFound,
		},
		"negative user": {
			userID:  -1,
			wantErr: model.ErrUserNotFound,
		},
		"missing user": {
			userID:  7,
			raw:     "18",
			number:  "18",
			wantErr: model.ErrUserNotFound,
		},
		"existence failure": {
			userID:    7,
			raw:       "18",
			number:    "18",
			existsErr: storageErr,
			wantErr:   storageErr,
		},
		"other owner": {
			userID:  7,
			raw:     "18",
			number:  "18",
			exists:  true,
			addErr:  model.ErrOrderOwnedByAnotherUser,
			wantErr: model.ErrOrderOwnedByAnotherUser,
		},
		"deleted before insert": {
			userID:  7,
			raw:     "18",
			number:  "18",
			exists:  true,
			addErr:  model.ErrUserNotFound,
			wantErr: model.ErrUserNotFound,
		},
		"insert failure": {
			userID:  7,
			raw:     "18",
			number:  "18",
			exists:  true,
			addErr:  storageErr,
			wantErr: storageErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage := mock.NewMockDBStorage(ctrl)
			validator := NewMockOrderNumberChecker(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			var calls []any
			if tc.userID > 0 {
				calls = append(calls, validator.EXPECT().Validate(tc.number).Return(tc.validationErr))
				if tc.validationErr == nil {
					calls = append(calls, storage.EXPECT().UserExists(ctx, tc.userID).
						Return(tc.exists, tc.existsErr))
					if tc.exists && tc.existsErr == nil {
						calls = append(calls, storage.EXPECT().AddOrder(ctx, tc.userID, tc.number).
							Return(tc.created, tc.addErr))
					}
				}
			}

			gomock.InOrder(calls...)

			created, err := service.NewOrder(storage, validator).Upload(ctx, tc.userID, tc.raw)
			require.ErrorIs(t, err, tc.wantErr)
			require.Equal(t, tc.created && tc.wantErr == nil, created)
		})
	}
}

func TestOrderList(t *testing.T) {
	storageErr := errors.New("storage failure")
	orders := []model.Order{{Number: "18"}, {Number: "0"}}
	tests := map[string]struct {
		userID    int64
		exists    bool
		existsErr error
		orders    []model.Order
		listErr   error
		wantErr   error
	}{
		"success": {
			userID: 7,
			exists: true,
			orders: orders,
		},
		"empty": {
			userID: 7,
			exists: true,
		},
		"zero": {
			wantErr: model.ErrUserNotFound,
		},
		"negative": {
			userID:  -7,
			wantErr: model.ErrUserNotFound,
		},
		"missing": {
			userID:  7,
			wantErr: model.ErrUserNotFound,
		},
		"existence error": {
			userID:    7,
			existsErr: storageErr,
			wantErr:   storageErr,
		},
		"list error discards partial data": {
			userID:  7,
			exists:  true,
			orders:  orders,
			listErr: storageErr,
			wantErr: storageErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage := mock.NewMockDBStorage(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tc.userID > 0 {
				check := storage.EXPECT().UserExists(ctx, tc.userID).Return(tc.exists, tc.existsErr)
				if tc.exists && tc.existsErr == nil {
					storage.EXPECT().ListOrdersByUser(ctx, tc.userID).Return(tc.orders, tc.listErr).
						After(check)
				}
			}

			got, err := service.NewOrder(storage, service.NewOrderNumberValidator()).List(ctx, tc.userID)
			require.ErrorIs(t, err, tc.wantErr)
			if tc.wantErr != nil {
				require.Nil(t, got)
				return
			}

			require.Equal(t, tc.orders, got)
		})
	}
}

func TestOrderPreservesCancellation(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage := mock.NewMockDBStorage(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage.EXPECT().UserExists(ctx, int64(7)).Return(false, context.Canceled)
	_, err := service.NewOrder(storage, service.NewOrderNumberValidator()).Upload(ctx, 7, "18")
	require.ErrorIs(t, err, context.Canceled)
}

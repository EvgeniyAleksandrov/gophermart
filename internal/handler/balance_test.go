package handler_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/handler"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestBalanceHandler(t *testing.T) {
	for name, tc := range map[string]struct {
		userID  int64
		balance model.Balance
		err     error
		status  int
		body    string
	}{
		"zero": {
			userID: 7,
			status: 200,
			body:   `{"current":0.00,"withdrawn":0.00}`,
		},
		"exact": {
			userID:  7,
			balance: model.Balance{Current: math.MaxInt64, Withdrawn: 42},
			status:  200,
			body:    `{"current":92233720368547758.07,"withdrawn":0.42}`,
		},
		"missing context": {
			status: 401,
			body:   "unauthorized\n",
		},
		"deleted user": {
			userID: 7,
			err:    model.ErrUserNotFound,
			status: 401,
			body:   "unauthorized\n",
		},
		"database failure": {
			userID: 7,
			err:    errors.New("secret SQL details"),
			status: 500,
			body:   "internal server error\n",
		},
		"invalid stored amount": {
			userID:  7,
			balance: model.Balance{Current: -1},
			status:  500,
			body:    "internal server error\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewMockBalanceService(gomock.NewController(t))
			var operationCtx context.Context
			if tc.userID != 0 {
				service.EXPECT().Get(gomock.Any(), tc.userID).DoAndReturn(func(ctx context.Context, id int64) (model.Balance, error) {
					operationCtx = ctx
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					require.InDelta(t, 25, time.Until(deadline).Seconds(), 0.1)
					return tc.balance, tc.err
				})
			}

			request := httptest.NewRequest(http.MethodGet, "/api/user/balance?user_id=99", nil)
			if tc.userID != 0 {
				request = request.WithContext(requestcontext.NewUser().WithID(request.Context(), tc.userID))
			}

			response := httptest.NewRecorder()
			handler.NewBalance(service, zap.NewNop()).ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, tc.body, response.Body.String())
			require.Empty(t, response.Header().Get("Authorization"))
			if tc.status == 200 {
				require.Equal(t, "application/json", response.Header().Get("Content-Type"))
			}

			if operationCtx != nil {
				require.ErrorIs(t, operationCtx.Err(), context.Canceled)
			}
		})
	}
}

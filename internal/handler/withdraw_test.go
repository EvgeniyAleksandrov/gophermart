package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/handler"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestWithdrawHandler(t *testing.T) {
	valid := `{"order":"18","sum":12.34}`

	for name, tc := range map[string]struct {
		body     string
		noUser   bool
		call     bool
		order    string
		err      error
		status   int
		response string
	}{
		"success": {
			body:   valid,
			call:   true,
			order:  "18",
			status: 200,
		},
		"ignored fields and content type": {
			body:   `{"order":"18","sum":12.340,"user_id":99}`,
			call:   true,
			order:  "18",
			status: 200,
		},
		"duplicates": {
			body:   `{"order":"0","order":"18","sum":0,"sum":1.234e1}`,
			call:   true,
			order:  "18",
			status: 200,
		},
		"exact limit": {
			body:   valid + strings.Repeat(" ", 64*1024-len(valid)),
			call:   true,
			order:  "18",
			status: 200,
		},
		"over limit": {
			body:     valid + strings.Repeat(" ", 64*1024-len(valid)+1),
			status:   400,
			response: "invalid request\n",
		},
		"no user": {
			body:     valid,
			noUser:   true,
			status:   401,
			response: "unauthorized\n",
		},
		"empty": {
			status:   400,
			response: "invalid request\n",
		},
		"second JSON takes precedence": {
			body:     `{"order":"18","sum":0} {}`,
			status:   400,
			response: "invalid request\n",
		},
		"missing order takes precedence": {
			body:     `{"sum":0}`,
			status:   400,
			response: "invalid request\n",
		},
		"null order": {
			body:     `{"order":null,"sum":12.34}`,
			status:   400,
			response: "invalid request\n",
		},
		"number order": {
			body:     `{"order":18,"sum":12.34}`,
			status:   400,
			response: "invalid request\n",
		},
		"string sum": {
			body:     `{"order":"18","sum":"12.34"}`,
			status:   400,
			response: "invalid request\n",
		},
		"null sum": {
			body:     `{"order":"18","sum":null}`,
			status:   400,
			response: "invalid request\n",
		},
		"invalid amount": {
			body:     `{"order":"18","sum":12.345}`,
			status:   422,
			response: "invalid withdrawal amount\n",
		},
		"amount before number": {
			body:     `{"order":"","sum":0}`,
			status:   422,
			response: "invalid withdrawal amount\n",
		},
		"empty number": {
			body:     `{"order":"","sum":12.34}`,
			call:     true,
			err:      model.ErrInvalidOrderNumber,
			status:   422,
			response: "invalid order number\n",
		},
		"service amount": {
			body:     valid,
			call:     true,
			order:    "18",
			err:      model.ErrInvalidWithdrawalAmount,
			status:   422,
			response: "invalid withdrawal amount\n",
		},
		"deleted user": {
			body:     valid,
			call:     true,
			order:    "18",
			err:      model.ErrUserNotFound,
			status:   401,
			response: "unauthorized\n",
		},
		"insufficient": {
			body:     valid,
			call:     true,
			order:    "18",
			err:      model.ErrInsufficientFunds,
			status:   402,
			response: "insufficient funds\n",
		},
		"conflict": {
			body:     valid,
			call:     true,
			order:    "18",
			err:      model.ErrWithdrawalConflict,
			status:   409,
			response: "withdrawal conflict\n",
		},
		"internal": {
			body:     valid,
			call:     true,
			order:    "18",
			err:      errors.New("private SQL details"),
			status:   500,
			response: "internal server error\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewMockWithdrawalService(gomock.NewController(t))
			var operationCtx context.Context
			if tc.call {
				service.EXPECT().Withdraw(gomock.Any(), int64(7), tc.order, int64(1234)).
					DoAndReturn(func(ctx context.Context, _ int64, _ string, _ int64) error {
						operationCtx = ctx
						deadline, ok := ctx.Deadline()
						require.True(t, ok)
						require.InDelta(t, 25, time.Until(deadline).Seconds(), 0.1)

						return tc.err
					})
			}

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/user/balance/withdraw?user_id=99",
				strings.NewReader(tc.body),
			)
			request.Header.Set("Content-Type", "text/plain")
			if !tc.noUser {
				request = request.WithContext(requestcontext.NewUser().WithID(request.Context(), 7))
			}

			response := httptest.NewRecorder()
			handler.NewWithdraw(service, zap.NewNop()).ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, tc.response, response.Body.String())
			require.Empty(t, response.Header().Get("Authorization"))
			if tc.status != 200 {
				require.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
			}

			if operationCtx != nil {
				require.ErrorIs(t, operationCtx.Err(), context.Canceled)
			}
		})
	}
}

func TestWithdrawContext(t *testing.T) {
	for name, tc := range map[string]struct {
		cancel bool
		want   error
	}{
		"timeout": {
			want: context.DeadlineExceeded,
		},
		"parent cancellation": {
			cancel: true,
			want:   context.Canceled,
		},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				service := NewMockWithdrawalService(gomock.NewController(t))
				service.EXPECT().Withdraw(gomock.Any(), int64(7), "18", int64(100)).
					DoAndReturn(func(ctx context.Context, _ int64, _ string, _ int64) error {
						<-ctx.Done()
						require.ErrorIs(t, ctx.Err(), tc.want)
						return ctx.Err()
					})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				if tc.cancel {
					cancel()
				}

				request := httptest.NewRequest(
					http.MethodPost,
					"/",
					strings.NewReader(`{"order":"18","sum":1}`),
				)
				request = request.WithContext(requestcontext.NewUser().WithID(ctx, 7))
				response := httptest.NewRecorder()
				handler.NewWithdraw(service, zap.NewNop()).ServeHTTP(response, request)
				require.Equal(t, 500, response.Code)
			})
		})
	}
}

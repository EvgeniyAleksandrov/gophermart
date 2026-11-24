package handler_test

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/auth"
	"github.com/EvgeniyAleksandrov/gophermart/internal/handler"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces/mock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/middleware"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestListWithdrawalsHandler(t *testing.T) {
	stamp := time.Date(2026, 11, 24, 3, 0, 0, 0, time.FixedZone("MSK", 3*60*60))

	for name, tc := range map[string]struct {
		rows   []model.Withdrawal
		err    error
		status int
		body   string
	}{
		"exact history": {
			rows: []model.Withdrawal{{
				ID:          1,
				UserID:      7,
				OrderNumber: "18",
				Sum:         math.MaxInt64,
				ProcessedAt: stamp,
			}},
			status: 200,
			body:   `[{"order":"18","sum":92233720368547758.07,"processed_at":"2026-11-24T00:00:00Z"}]`,
		},
		"empty": {
			status: 204,
		},
		"deleted user": {
			err:    model.ErrUserNotFound,
			status: 401,
			body:   "unauthorized\n",
		},
		"storage failure": {
			err:    errors.New("private SQL details"),
			status: 500,
			body:   "internal server error\n",
		},
		"presentation failure before headers": {
			rows:   []model.Withdrawal{{OrderNumber: "18", Sum: -1}},
			status: 500,
			body:   "internal server error\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewMockWithdrawalService(gomock.NewController(t))
			var operation context.Context
			service.EXPECT().List(gomock.Any(), int64(7)).
				DoAndReturn(func(ctx context.Context, _ int64) ([]model.Withdrawal, error) {
					operation = ctx
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					require.InDelta(t, 25, time.Until(deadline).Seconds(), 0.1)

					return tc.rows, tc.err
				})
			request := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals?user_id=99", nil)
			request = request.WithContext(requestcontext.NewUser().WithID(request.Context(), 7))
			response := httptest.NewRecorder()
			handler.NewListWithdrawals(service, zap.NewNop()).ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, tc.body, response.Body.String())
			require.Empty(t, response.Header().Get("Authorization"))
			require.ErrorIs(t, operation.Err(), context.Canceled)
			if tc.status == 200 {
				require.Equal(t, "application/json", response.Header().Get("Content-Type"))
			}
		})
	}
}

func TestListWithdrawalsWriteFailure(t *testing.T) {
	service := NewMockWithdrawalService(gomock.NewController(t))
	service.EXPECT().List(gomock.Any(), int64(7)).Return([]model.Withdrawal{{OrderNumber: "18", Sum: 1}}, nil)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(requestcontext.NewUser().WithID(request.Context(), 7))
	writer := &failedOrderWriter{header: make(http.Header)}
	handler.NewListWithdrawals(service, zap.NewNop()).ServeHTTP(writer, request)
	require.Equal(t, []int{200}, writer.statuses)
	require.Equal(t, 1, writer.writes)
}

func TestWithdrawalHandlersJWTAndDeletedUser(t *testing.T) {
	tokens, err := auth.NewTokenManager([]byte(strings.Repeat("k", 32)), auth.NewSystemClock())
	require.NoError(t, err)
	token, err := tokens.Issue(7)
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		authorization string
		deleted       bool
		status        int
	}{
		"missing JWT": {
			status: 401,
		},
		"invalid JWT": {
			authorization: "Bearer invalid",
			status:        401,
		},
		"deleted user with valid JWT": {
			authorization: "Bearer " + token,
			deleted:       true,
			status:        401,
		},
		"valid JWT owner isolation": {
			authorization: "Bearer " + token,
			status:        200,
		},
	} {
		t.Run(name, func(t *testing.T) {
			for method, path := range map[string]string{
				http.MethodPost: "/api/user/balance/withdraw",
				http.MethodGet:  "/api/user/withdrawals",
			} {
				t.Run(method, func(t *testing.T) {
					storage := mock.NewMockDBStorage(gomock.NewController(t))
					if tc.authorization == "Bearer "+token {
						check := storage.EXPECT().UserExists(gomock.Any(), int64(7)).Return(!tc.deleted, nil)
						if !tc.deleted {
							if method == http.MethodPost {
								storage.EXPECT().Withdraw(gomock.Any(), int64(7), "18", int64(100)).
									Return(nil).
									After(check)
							} else {
								storage.EXPECT().ListWithdrawalsByUser(gomock.Any(), int64(7)).
									Return([]model.Withdrawal{{OrderNumber: "18", Sum: 100}}, nil).
									After(check)
							}
						}
					}

					svc := service.NewWithdrawal(storage, service.NewOrderNumberValidator())
					var endpoint http.Handler = handler.NewWithdraw(svc, zap.NewNop())
					if method == http.MethodGet {
						endpoint = handler.NewListWithdrawals(svc, zap.NewNop())
					}

					request := httptest.NewRequest(
						method,
						path+"?user_id=99",
						strings.NewReader(`{"order":"18","sum":1,"user_id":99}`),
					)
					if tc.authorization != "" {
						request.Header.Set("Authorization", tc.authorization)
					}

					response := httptest.NewRecorder()
					middleware.NewAuth(tokens).Wrap(endpoint).ServeHTTP(response, request)
					require.Equal(t, tc.status, response.Code)
					require.Empty(t, response.Header().Get("Authorization"))
				})
			}
		})
	}
}

func TestWithdrawalHandlersRequirePositiveContextID(t *testing.T) {
	for name, tc := range map[string]struct {
		id     int64
		attach bool
	}{
		"missing": {},
		"zero": {
			attach: true,
		},
		"negative": {
			id:     -1,
			attach: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			service := NewMockWithdrawalService(gomock.NewController(t))

			for _, endpoint := range []http.Handler{
				handler.NewWithdraw(service, zap.NewNop()),
				handler.NewListWithdrawals(service, zap.NewNop()),
			} {
				request := httptest.NewRequest(http.MethodGet, "/", nil)
				if tc.attach {
					request = request.WithContext(requestcontext.NewUser().WithID(request.Context(), tc.id))
				}

				response := httptest.NewRecorder()
				endpoint.ServeHTTP(response, request)
				require.Equal(t, 401, response.Code)
			}
		})
	}
}

func TestListWithdrawalsContext(t *testing.T) {
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
				service.EXPECT().List(gomock.Any(), int64(7)).
					DoAndReturn(func(ctx context.Context, _ int64) ([]model.Withdrawal, error) {
						<-ctx.Done()
						require.ErrorIs(t, ctx.Err(), tc.want)
						return nil, ctx.Err()
					})
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				if tc.cancel {
					cancel()
				}

				request := httptest.NewRequest(http.MethodGet, "/", nil).
					WithContext(requestcontext.NewUser().WithID(ctx, 7))
				response := httptest.NewRecorder()
				handler.NewListWithdrawals(service, zap.NewNop()).ServeHTTP(response, request)
				require.Equal(t, 500, response.Code)
			})
		})
	}
}

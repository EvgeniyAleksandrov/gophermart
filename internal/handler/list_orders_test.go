package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/auth"
	"github.com/EvgeniyAleksandrov/gophermart/internal/handler"
	"github.com/EvgeniyAleksandrov/gophermart/internal/middleware"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestListOrdersResponses(t *testing.T) {
	negative := int64(-1)
	tests := map[string]struct {
		orders []model.Order
		err    error
		code   int
		body   string
	}{
		"empty": {
			code: http.StatusNoContent,
		},
		"missing user": {
			err:  model.ErrUserNotFound,
			code: http.StatusUnauthorized,
			body: "unauthorized\n",
		},
		"storage failure": {
			err:  errors.New("secret storage detail"),
			code: http.StatusInternalServerError,
			body: "internal server error\n",
		},
		"presentation failure": {
			orders: []model.Order{{Accrual: &negative}},
			code:   http.StatusInternalServerError,
			body:   "internal server error\n",
		},
		"ordered array": {
			orders: []model.Order{
				{
					ID:         20,
					UserID:     7,
					Number:     "18",
					Status:     model.OrderStatusNew,
					UploadedAt: time.Date(2026, 11, 22, 12, 0, 0, 0, time.FixedZone("MSK", 10800)),
				},
				{
					ID:         19,
					UserID:     7,
					Number:     "0",
					Status:     model.OrderStatusProcessing,
					UploadedAt: time.Date(2026, 11, 22, 8, 0, 0, 0, time.UTC),
				},
			},
			code: http.StatusOK,
			body: `[{"number":"18","status":"NEW","uploaded_at":"2026-11-22T09:00:00Z"},{"number":"0","status":"PROCESSING","uploaded_at":"2026-11-22T08:00:00Z"}]`,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewMockOrderService(gomock.NewController(t))
			svc.EXPECT().List(gomock.Any(), int64(7)).Return(tc.orders, tc.err)
			req := httptest.NewRequest(http.MethodGet, "/api/user/orders?user_id=99", strings.NewReader(`{"user_id":99}`))
			req = req.WithContext(requestcontext.NewUser().WithID(req.Context(), 7))
			rec := httptest.NewRecorder()
			handler.NewListOrders(svc, zap.NewNop()).ServeHTTP(rec, req)
			require.Equal(t, tc.code, rec.Code)
			require.Equal(t, tc.body, rec.Body.String())
			require.Empty(t, rec.Header().Get("Authorization"))
			switch tc.code {
			case http.StatusOK:
				require.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			case http.StatusNoContent:
				require.Empty(t, rec.Header().Get("Content-Type"))
			default:
				require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestListOrdersAuthorization(t *testing.T) {
	tokens, err := auth.NewTokenManager([]byte(strings.Repeat("k", 32)), auth.NewSystemClock())
	require.NoError(t, err)
	valid, err := tokens.Issue(7)
	require.NoError(t, err)
	tests := map[string]struct {
		header string
		code   int
	}{
		"absent": {
			code: http.StatusUnauthorized,
		},
		"invalid": {
			header: "Bearer bad",
			code:   http.StatusUnauthorized,
		},
		"valid": {
			header: "Bearer " + valid,
			code:   http.StatusNoContent,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewMockOrderService(gomock.NewController(t))
			if tc.code == http.StatusNoContent {
				svc.EXPECT().List(gomock.Any(), int64(7)).Return(nil, nil)
			}
			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()
			middleware.NewAuth(tokens).Wrap(handler.NewListOrders(svc, zap.NewNop())).ServeHTTP(rec, req)
			require.Equal(t, tc.code, rec.Code)
			require.Empty(t, rec.Header().Get("Authorization"))
		})
	}
}

func TestListOrdersContext(t *testing.T) {
	for name, tc := range map[string]struct {
		early    bool
		canceled bool
	}{
		"budget": {},
		"parent deadline": {
			early: true,
		},
		"parent cancellation": {
			canceled: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			parent := context.Background()
			if tc.early {
				var cancel context.CancelFunc
				parent, cancel = context.WithTimeout(parent, time.Second)
				defer cancel()
			}
			if tc.canceled {
				var cancel context.CancelFunc
				parent, cancel = context.WithCancel(parent)
				cancel()
			}
			svc := NewMockOrderService(gomock.NewController(t))
			var operation context.Context
			before := time.Now()
			svc.EXPECT().List(gomock.Any(), int64(7)).DoAndReturn(func(ctx context.Context, id int64) ([]model.Order, error) {
				operation = ctx
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				if tc.early {
					expected, _ := parent.Deadline()
					require.Equal(t, expected, deadline)
				} else {
					require.WithinDuration(t, before.Add(25*time.Second), deadline, time.Second)
				}
				if tc.canceled {
					require.ErrorIs(t, ctx.Err(), context.Canceled)
					return nil, ctx.Err()
				}
				return nil, nil
			})
			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			req = req.WithContext(requestcontext.NewUser().WithID(parent, 7))
			rec := httptest.NewRecorder()
			handler.NewListOrders(svc, zap.NewNop()).ServeHTTP(rec, req)
			require.ErrorIs(t, operation.Err(), context.Canceled)
			if tc.canceled {
				require.Equal(t, http.StatusInternalServerError, rec.Code)
			} else {
				require.Equal(t, http.StatusNoContent, rec.Code)
			}
		})
	}
}

func TestOrderHandlersRequireContextUser(t *testing.T) {
	for name, tc := range map[string]struct {
		attach bool
		id     int64
	}{
		"absent": {},
		"zero": {
			attach: true,
		},
		"negative": {
			attach: true,
			id:     -1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			svc := NewMockOrderService(gomock.NewController(t))
			for method, h := range map[string]http.Handler{
				http.MethodGet:  handler.NewListOrders(svc, zap.NewNop()),
				http.MethodPost: handler.NewUploadOrder(svc, zap.NewNop()),
			} {
				req := httptest.NewRequest(method, "/api/user/orders?user_id=7", strings.NewReader("18"))
				if tc.attach {
					req = req.WithContext(requestcontext.NewUser().WithID(req.Context(), tc.id))
				}
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				require.Equal(t, http.StatusUnauthorized, rec.Code)
				require.Equal(t, "unauthorized\n", rec.Body.String())
			}
		})
	}
}

// failedOrderWriter simulates a disconnected client after headers were sent.
type failedOrderWriter struct {
	header   http.Header
	statuses []int
	writes   int
}

func (w *failedOrderWriter) Header() http.Header {
	return w.header
}

func (w *failedOrderWriter) WriteHeader(code int) {
	w.statuses = append(w.statuses, code)
}

func (w *failedOrderWriter) Write(body []byte) (int, error) {
	w.writes++
	return 0, errors.New("connection lost")
}

func TestListOrdersWriteFailure(t *testing.T) {
	svc := NewMockOrderService(gomock.NewController(t))
	svc.EXPECT().List(gomock.Any(), int64(7)).Return([]model.Order{{Number: "18", Status: model.OrderStatusNew}}, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req = req.WithContext(requestcontext.NewUser().WithID(req.Context(), 7))
	writer := &failedOrderWriter{header: make(http.Header)}
	handler.NewListOrders(svc, zap.NewNop()).ServeHTTP(writer, req)
	require.Equal(t, []int{http.StatusOK}, writer.statuses)
	require.Equal(t, 1, writer.writes)
}

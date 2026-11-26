package worker_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/client/accrual"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces/mock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/EvgeniyAleksandrov/gophermart/internal/worker"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestAccrualQueue(t *testing.T) {
	for name, tc := range map[string]struct {
		empty      bool
		pendingErr error
		processErr error
	}{
		"sequential": {},
		"empty still waits": {
			empty: true,
		},
		"pending failure recovers": {
			pendingErr: errors.New("DB failed"),
		},
		"external failure continues": {
			processErr: model.ErrAccrualUnavailable,
		},
		"204 continues": {
			processErr: model.ErrAccrualNotRegistered,
		},
		"invalid response continues": {
			processErr: model.ErrAccrualInvalidResponse,
		},
		"persistence failure aborts pass": {
			processErr: &model.PersistenceError{Err: errors.New("DB failed")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				processor := NewMockProcessor(gomock.NewController(t))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				started := time.Now()
				orders := []model.Order{{ID: 1}, {ID: 2}}
				if tc.empty {
					orders = nil
				}

				calls := []any{processor.EXPECT().Pending(ctx).
					DoAndReturn(func(context.Context) ([]model.Order, error) {
						require.Zero(t, time.Since(started), "first pass must start immediately")
						return orders, tc.pendingErr
					})}
				if !tc.empty && tc.pendingErr == nil {
					calls = append(calls, processor.EXPECT().Process(ctx, orders[0]).Return(tc.processErr))
					var persistence *model.PersistenceError
					if !errors.As(tc.processErr, &persistence) {
						calls = append(calls, processor.EXPECT().Process(ctx, orders[1]).Return(nil))
					}
				}

				calls = append(calls, processor.EXPECT().Pending(ctx).
					DoAndReturn(func(context.Context) ([]model.Order, error) {
						require.Equal(
							t,
							time.Second,
							time.Since(started),
							"next pass follows the poll interval",
						)
						cancel()
						return nil, nil
					}))
				gomock.InOrder(calls...)
				require.NoError(t, worker.NewAccrual(processor, zap.NewNop()).Run(ctx))
			})
		})
	}
}

func TestAccrualCancellation(t *testing.T) {
	for name, tc := range map[string]struct {
		before        bool
		duringRequest bool
	}{
		"before startup": {
			before: true,
		},
		"during request": {
			duringRequest: true,
		},
		"during poll wait": {},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				processor := NewMockProcessor(gomock.NewController(t))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				order := model.Order{ID: 1}

				switch {
				case tc.before:
					cancel()
				case tc.duringRequest:
					processor.EXPECT().Pending(ctx).Return([]model.Order{order}, nil)
					processor.EXPECT().Process(ctx, order).
						DoAndReturn(func(context.Context, model.Order) error { cancel(); return ctx.Err() })
				default:
					processor.EXPECT().Pending(ctx).Return(nil, nil)
				}

				done := make(chan error, 1)
				go func() { done <- worker.NewAccrual(processor, zap.NewNop()).Run(ctx) }()
				synctest.Wait()
				cancel()
				require.NoError(t, <-done)
			})
		})
	}
}

func TestAccrualRateLimitKeepsQueueOnSameOrder(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage := mock.NewMockDBStorage(ctrl)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	orders := []model.Order{{ID: 1, UserID: 7, Number: "18"}, {ID: 2, UserID: 7, Number: "26"}}
	storage.EXPECT().ListPending(ctx).Return(orders, nil)
	gomock.InOrder(
		storage.EXPECT().
			ApplyAccrual(
				ctx,
				int64(1),
				int64(7),
				model.AccrualResult{Number: "18", Status: model.AccrualProcessed},
			).Return(nil),
		storage.EXPECT().
			ApplyAccrual(
				ctx,
				int64(2),
				int64(7),
				model.AccrualResult{Number: "26", Status: model.AccrualProcessed},
			).DoAndReturn(func(
			context.Context,
			int64,
			int64,
			model.AccrualResult,
		) error {
			cancel()
			return nil
		}),
	)

	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if len(paths) <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}

		number := strings.TrimPrefix(r.URL.Path, "/api/orders/")
		fmt.Fprintf(w, `{"order":%q,"status":"PROCESSED"}`, number)
	}))
	defer server.Close()

	client := accrual.NewClient(server.URL, server.Client(), retry.NewRetryer(strategy.NewRateLimit()))
	processor := service.NewAccrual(storage, client)
	require.NoError(t, worker.NewAccrual(processor, zap.NewNop()).Run(ctx))
	require.Equal(t, []string{"/api/orders/18", "/api/orders/18", "/api/orders/18", "/api/orders/26"}, paths)
}

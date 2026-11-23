package retry_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestRetryerDo(t *testing.T) {
	failure := &pgconn.PgError{Code: "40001"}
	for name, tc := range map[string]struct {
		failures     int
		operationErr error
		wantCalls    int
		wantErr      error
	}{
		"success": {
			wantCalls: 1,
		},
		"retry then success": {
			failures:     2,
			operationErr: failure,
			wantCalls:    3,
		},
		"exhausted returns last error": {
			failures:     10,
			operationErr: failure,
			wantCalls:    5,
			wantErr:      failure,
		},
		"unmatched returns original error": {
			failures:     1,
			operationErr: operationFailure,
			wantCalls:    1,
			wantErr:      operationFailure,
		},
	} {
		t.Run(name, func(t *testing.T) {
			policy, err := strategy.NewExponential(5, 0, time.Second, strategy.NewSQLState([]string{"40001"}))
			require.NoError(t, err)
			calls := 0
			err = retry.NewRetryer(policy).Do(context.Background(), func() error {
				calls++
				if calls <= tc.failures {
					return tc.operationErr
				}
				return nil
			})
			require.Equal(t, tc.wantCalls, calls)
			require.True(t, tc.wantErr == err, "must return the original error, including nil")
		})
	}
}

var operationFailure = errors.New("operation failed")

func TestRetryerSafetyBeforeStrategy(t *testing.T) {
	failure := errors.New("uncertain write")
	for name, tc := range map[string]struct {
		err error
	}{
		"canceled error": {
			err: fmt.Errorf("operation: %w", context.Canceled),
		},
		"deadline error": {
			err: fmt.Errorf("operation: %w", context.DeadlineExceeded),
		},
		"permanent error": {
			err: errors.Join(errors.New("cleanup"), &retry.PermanentError{Err: failure}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			policy := NewMockStrategy(gomock.NewController(t)) // No Next call is allowed.
			calls := 0
			err := retry.NewRetryer(policy).Do(context.Background(), func() error { calls++; return tc.err })
			require.Same(t, tc.err, err)
			require.Equal(t, 1, calls)
			if errors.Is(tc.err, failure) {
				require.ErrorIs(t, err, failure)
				require.Contains(t, err.Error(), "uncertain write")
			}
		})
	}
}

func TestRetryerCancellation(t *testing.T) {
	for name, tc := range map[string]struct {
		before          bool
		duringOperation bool
		deadline        bool
	}{
		"before call": {
			before: true,
		},
		"during operation": {
			duringOperation: true,
		},
		"during wait": {},
		"deadline during wait": {
			deadline: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				policy := NewMockStrategy(gomock.NewController(t))
				var ctx context.Context
				var cancel context.CancelFunc
				if tc.deadline {
					ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
				} else {
					ctx, cancel = context.WithCancel(context.Background())
				}
				defer cancel()
				if tc.before {
					cancel()
				}
				if !tc.before && !tc.duringOperation {
					policy.EXPECT().Next(1, operationFailure).Return(time.Hour, true)
				}
				calls := 0
				done := make(chan error, 1)
				go func() {
					done <- retry.NewRetryer(policy).Do(ctx, func() error {
						calls++
						if tc.duringOperation {
							cancel()
						}
						return operationFailure
					})
				}()
				synctest.Wait()
				if !tc.deadline {
					cancel()
				}
				err := <-done
				if tc.deadline {
					require.ErrorIs(t, err, context.DeadlineExceeded)
				} else {
					require.ErrorIs(t, err, context.Canceled)
				}
				if tc.before {
					require.Zero(t, calls)
				} else {
					require.Equal(t, 1, calls)
				}
			})
		})
	}
}

func TestRetryerWaitsAndReturnsLastError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		policy := NewMockStrategy(gomock.NewController(t))
		first, last := errors.New("first attempt"), errors.New("last attempt")
		gomock.InOrder(policy.EXPECT().Next(1, first).Return(2*time.Second, true), policy.EXPECT().Next(2, last).Return(time.Duration(0), false))
		started, calls := time.Now(), 0
		err := retry.NewRetryer(policy).Do(context.Background(), func() error {
			calls++
			if calls == 1 {
				return first
			}
			require.Equal(t, 2*time.Second, time.Since(started))
			return last
		})
		require.Same(t, last, err)
	})
}

func TestRetryerConcurrentCallsHaveSeparateBudgets(t *testing.T) {
	policy, err := strategy.NewExponential(5, 0, time.Second, strategy.NewSQLState([]string{"40001"}))
	require.NoError(t, err)
	runner := retry.NewRetryer(policy)
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Go(func() {
			calls := 0
			failure := &pgconn.PgError{Code: "40001"}
			err := runner.Do(context.Background(), func() error { calls++; return failure })
			require.Same(t, failure, err)
			require.Equal(t, 5, calls)
		})
	}
	group.Wait()
}

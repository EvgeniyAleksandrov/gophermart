package postgres_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/repository/postgres"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestApplyAccrual(t *testing.T) {
	failure := errors.New("SQL failed")

	for name, tc := range map[string]struct {
		status         model.AccrualStatus
		final          bool
		missingOrder   bool
		missingBalance bool
		userExists     bool
		current        int64
		amount         int64
		beginErr       error
		lockErr        error
		orderErr       error
		balanceErr     error
		updateErr      error
		commitErr      error
		rollbackErr    error
		mismatch       bool
		wantErr        bool
	}{
		"processed": {
			status:  model.AccrualProcessed,
			current: 100,
			amount:  125,
		},
		"zero": {
			status: model.AccrualProcessed,
		},
		"registered": {
			status: model.AccrualRegistered,
		},
		"processing": {
			status: model.AccrualProcessing,
		},
		"invalid": {
			status: model.AccrualInvalid,
		},
		"final unchanged": {
			final: true,
		},
		"deleted order": {
			missingOrder: true,
		},
		"deleted user": {
			missingBalance: true,
		},
		"missing mandatory balance": {
			missingBalance: true,
			userExists:     true,
			wantErr:        true,
		},
		"overflow": {
			status:  model.AccrualProcessed,
			current: math.MaxInt64,
			amount:  1,
			wantErr: true,
		},
		"negative": {
			status:  model.AccrualProcessed,
			amount:  -1,
			wantErr: true,
		},
		"begin error": {
			beginErr: failure,
			wantErr:  true,
		},
		"lock error": {
			lockErr: failure,
			wantErr: true,
		},
		"order error": {
			orderErr: failure,
			wantErr:  true,
		},
		"balance update error": {
			status:     model.AccrualProcessed,
			amount:     123,
			balanceErr: failure,
			wantErr:    true,
		},
		"order update error rolls balance back": {
			status:    model.AccrualProcessed,
			amount:    123,
			updateErr: failure,
			wantErr:   true,
		},
		"commit error": {
			status:    model.AccrualProcessed,
			commitErr: failure,
			wantErr:   true,
		},
		"rollback error": {
			lockErr:     failure,
			rollbackErr: errors.New("rollback failed"),
			wantErr:     true,
		},
		"identity mismatch": {
			mismatch: true,
			wantErr:  true,
		},
		"unknown status": {
			status:  "UNKNOWN",
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectBegin().WillReturnError(tc.beginErr)
			stopped := tc.beginErr != nil
			if !stopped {
				rows := sqlmock.NewRows([]string{"current"})
				if !tc.missingBalance {
					rows.AddRow(tc.current)
				}

				mock.ExpectQuery("SELECT current .*FOR UPDATE").WithArgs(int64(7)).WillReturnRows(rows).
					WillReturnError(tc.lockErr)
				stopped = tc.lockErr != nil || tc.missingBalance
				if tc.missingBalance {
					mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(7)).
						WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tc.userExists))
				}
			}

			if !stopped {
				rows := sqlmock.NewRows([]string{"user_id", "number", "status"})
				status := "NEW"
				if tc.final {
					status = "PROCESSED"
				}

				owner := int64(7)
				if tc.mismatch {
					owner = 8
				}

				if !tc.missingOrder {
					rows.AddRow(owner, "18", status)
				}

				mock.ExpectQuery("SELECT user_id, number, status .*FOR UPDATE").WithArgs(int64(1)).
					WillReturnRows(rows).
					WillReturnError(tc.orderErr)
				stopped = tc.missingOrder || tc.orderErr != nil || tc.final || tc.mismatch
			}

			next := "PROCESSING"
			var amount any
			if !stopped {
				switch tc.status {
				case model.AccrualRegistered, model.AccrualProcessing:
				case model.AccrualInvalid:
					next, amount = "INVALID", int64(0)
				case model.AccrualProcessed:
					next, amount = "PROCESSED", tc.amount
					stopped = tc.amount < 0 || tc.amount > math.MaxInt64-tc.current
					if !stopped {
						mock.ExpectExec("UPDATE public.gophermart_balances").
							WithArgs(tc.current+tc.amount, int64(7)).
							WillReturnResult(sqlmock.NewResult(0, 1)).
							WillReturnError(tc.balanceErr)
						stopped = tc.balanceErr != nil
					}
				default:
					stopped = true
				}
			}

			if !stopped {
				mock.ExpectExec("UPDATE public.gophermart_orders").WithArgs(next, amount, int64(1)).
					WillReturnResult(sqlmock.NewResult(0, 1)).
					WillReturnError(tc.updateErr)
				stopped = tc.updateErr != nil
			}

			if !stopped {
				mock.ExpectCommit().WillReturnError(tc.commitErr)
			} else if tc.beginErr == nil {
				mock.ExpectRollback().WillReturnError(tc.rollbackErr)
			}

			err = postgres.NewDBStorage(db, singleAttempt(t)).
				ApplyAccrual(
					context.Background(),
					1,
					7,
					model.AccrualResult{Number: "18", Status: tc.status, Accrual: tc.amount},
				)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			if tc.rollbackErr != nil {
				require.ErrorIs(t, err, tc.rollbackErr)
			}

			if tc.commitErr != nil {
				var unknown *postgres.OutcomeUnknownError
				require.ErrorAs(t, err, &unknown)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestApplyAccrualRetryFreshTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT current .*FOR UPDATE").WillReturnError(&pgconn.PgError{Code: "40P01"})
	mock.ExpectRollback()
	// A complete new transaction is required after rollback, never a nested query retry.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT current .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"current"}).AddRow(123))
	mock.ExpectQuery("SELECT user_id, number, status .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "number", "status"}).AddRow(7, "18", "PROCESSED"))
	mock.ExpectRollback()
	err = postgres.NewDBStorage(db, retryPolicy(t, 5)).
		ApplyAccrual(
			context.Background(),
			1,
			7,
			model.AccrualResult{Number: "18", Status: model.AccrualProcessed, Accrual: 123},
		)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyAccrualUnknownCommitWithExplicitPolicy(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	failure := &pgconn.PgError{Code: "08006", Message: "connection lost during commit"}
	policy, err := strategy.NewExponential(5, 0, time.Second, strategy.NewSQLState([]string{"08006"}))
	require.NoError(t, err)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT current .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"current"}).AddRow(0))
	mock.ExpectQuery("SELECT user_id, number, status .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "number", "status"}).AddRow(7, "18", "NEW"))
	mock.ExpectExec("UPDATE public.gophermart_balances").WithArgs(int64(123), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE public.gophermart_orders").WithArgs("PROCESSED", int64(123), int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit().WillReturnError(failure)
	// Simulate a commit that actually succeeded despite the lost acknowledgement.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT current .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"current"}).AddRow(123))
	mock.ExpectQuery("SELECT user_id, number, status .*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "number", "status"}).AddRow(7, "18", "PROCESSED"))
	mock.ExpectRollback()
	err = postgres.NewDBStorage(db, retry.NewRetryer(policy)).
		ApplyAccrual(
			context.Background(),
			1,
			7,
			model.AccrualResult{Number: "18", Status: model.AccrualProcessed, Accrual: 123},
		)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

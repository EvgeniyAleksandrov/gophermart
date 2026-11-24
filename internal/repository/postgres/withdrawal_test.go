package postgres_test

import (
	"context"
	"database/sql"
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

func TestWithdrawSQL(t *testing.T) {
	failure := errors.New("SQL failure")
	cleanup := errors.New("cleanup failure")

	for name, tc := range map[string]struct {
		setup    func(sqlmock.Sqlmock)
		want     error
		anyError bool
		cleanup  bool
	}{
		"success": {
			setup: func(m sqlmock.Sqlmock) {
				(&withdrawalSQL{m}).newDebit(10000, 0)
				m.ExpectCommit()
			},
		},
		"repeat before insufficient funds": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(0, 3000)
				q.existing(7, 3000)
				m.ExpectRollback()
			},
		},
		"other owner": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.existing(8, 3000)
				m.ExpectRollback()
			},
			want: model.ErrWithdrawalConflict,
		},
		"other sum": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.existing(7, 3001)
				m.ExpectRollback()
			},
			want: model.ErrWithdrawalConflict,
		},
		"insert conflict matching repeat": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(0, 3000)
				q.absent()
				q.insert().WillReturnError(sql.ErrNoRows)
				q.existing(7, 3000)
				m.ExpectRollback()
			},
		},
		"insert conflict other owner": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.absent()
				q.insert().WillReturnError(sql.ErrNoRows)
				q.existing(8, 3000)
				m.ExpectRollback()
			},
			want: model.ErrWithdrawalConflict,
		},
		"insert conflict other sum": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.absent()
				q.insert().WillReturnError(sql.ErrNoRows)
				q.existing(7, 3001)
				m.ExpectRollback()
			},
			want: model.ErrWithdrawalConflict,
		},
		"winner vanished": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.absent()
				q.insert().WillReturnError(sql.ErrNoRows)
				q.absent()
				m.ExpectRollback()
			},
			want: sql.ErrNoRows,
		},
		"winner scan error": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.absent()
				q.insert().WillReturnError(sql.ErrNoRows)
				m.ExpectQuery("SELECT user_id, sum").WithArgs("18").
					WillReturnRows(sqlmock.NewRows([]string{"user_id", "sum"}).AddRow(7, "invalid"))
				m.ExpectRollback()
			},
			anyError: true,
		},
		"insufficient rolls inserted history back": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(2999, 0)
				q.absent()
				q.insert().WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				m.ExpectRollback()
			},
			want: model.ErrInsufficientFunds,
		},
		"withdrawn overflow": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(3000, math.MaxInt64-2999)
				q.absent()
				q.insert().WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
				m.ExpectRollback()
			},
			anyError: true,
		},
		"withdrawn max and exact balance": {
			setup: func(m sqlmock.Sqlmock) {
				(&withdrawalSQL{m}).newDebit(3000, math.MaxInt64-3000)
				m.ExpectCommit()
			},
		},
		"missing user": {
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT current, withdrawn .*FOR UPDATE").WithArgs(int64(7)).
					WillReturnError(sql.ErrNoRows)
				m.ExpectQuery("SELECT EXISTS").WithArgs(int64(7)).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				m.ExpectRollback()
			},
			want: model.ErrUserNotFound,
		},
		"missing mandatory balance": {
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT current, withdrawn .*FOR UPDATE").WillReturnError(sql.ErrNoRows)
				m.ExpectQuery("SELECT EXISTS").
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				m.ExpectRollback()
			},
			want: model.ErrBalanceNotFound,
		},
		"user check error": {
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT current, withdrawn .*FOR UPDATE").WillReturnError(sql.ErrNoRows)
				m.ExpectQuery("SELECT EXISTS").WillReturnError(failure)
				m.ExpectRollback()
			},
			want: failure,
		},
		"begin failure": {
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin().WillReturnError(failure)
			},
			want: failure,
		},
		"lock failure with cleanup": {
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT current, withdrawn .*FOR UPDATE").WillReturnError(failure)
				m.ExpectRollback().WillReturnError(cleanup)
			},
			want:    failure,
			cleanup: true,
		},
		"balance scan failure": {
			setup: func(m sqlmock.Sqlmock) {
				m.ExpectBegin()
				m.ExpectQuery("SELECT current, withdrawn .*FOR UPDATE").
					WillReturnRows(sqlmock.NewRows([]string{"current", "withdrawn"}).AddRow(10000, "invalid"))
				m.ExpectRollback()
			},
			anyError: true,
		},
		"existing query failure": {
			setup: func(m sqlmock.Sqlmock) {
				(&withdrawalSQL{m}).balance(10000, 0)
				m.ExpectQuery("SELECT user_id, sum").WillReturnError(failure)
				m.ExpectRollback()
			},
			want: failure,
		},
		"existing scan failure": {
			setup: func(m sqlmock.Sqlmock) {
				(&withdrawalSQL{m}).balance(10000, 0)
				m.ExpectQuery("SELECT user_id, sum").
					WillReturnRows(sqlmock.NewRows([]string{"user_id", "sum"}).AddRow(7, "invalid"))
				m.ExpectRollback()
			},
			anyError: true,
		},
		"insert failure": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.absent()
				q.insert().WillReturnError(failure)
				m.ExpectRollback()
			},
			want: failure,
		},
		"insert scan failure": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(10000, 0)
				q.absent()
				q.insert().WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("invalid"))
				m.ExpectRollback()
			},
			anyError: true,
		},
		"update failure": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.inserted()
				q.update().WillReturnError(failure)
				m.ExpectRollback()
			},
			want: failure,
		},
		"rows affected failure": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.inserted()
				q.update().WillReturnResult(sqlmock.NewErrorResult(failure))
				m.ExpectRollback()
			},
			want: failure,
		},
		"zero rows updated": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.inserted()
				q.update().WillReturnResult(sqlmock.NewResult(0, 0))
				m.ExpectRollback()
			},
			anyError: true,
		},
		"two rows updated": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.inserted()
				q.update().WillReturnResult(sqlmock.NewResult(0, 2))
				m.ExpectRollback()
			},
			anyError: true,
		},
		"commit failure": {
			setup: func(m sqlmock.Sqlmock) {
				(&withdrawalSQL{m}).newDebit(10000, 0)
				m.ExpectCommit().WillReturnError(failure)
			},
			want: failure,
		},
		"repeat cleanup failure": {
			setup: func(m sqlmock.Sqlmock) {
				q := &withdrawalSQL{m}
				q.balance(0, 3000)
				q.existing(7, 3000)
				m.ExpectRollback().WillReturnError(cleanup)
			},
			want: cleanup,
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			tc.setup(mock)
			err = postgres.NewDBStorage(db, singleAttempt(t)).Withdraw(context.Background(), 7, "18", 3000)
			if tc.anyError {
				require.Error(t, err)
				require.NotErrorIs(t, err, model.ErrWithdrawalConflict)
				require.NotErrorIs(t, err, model.ErrInsufficientFunds)
				require.NotErrorIs(t, err, model.ErrInvalidWithdrawalAmount)
			} else {
				require.ErrorIs(t, err, tc.want)
			}

			if tc.cleanup {
				require.ErrorIs(t, err, cleanup)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

type withdrawalSQL struct {
	mock sqlmock.Sqlmock
}

func (q *withdrawalSQL) balance(current, withdrawn int64) {
	q.mock.ExpectBegin()
	q.mock.ExpectQuery("SELECT current, withdrawn .*FOR UPDATE").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"current", "withdrawn"}).AddRow(current, withdrawn))
}

func (q *withdrawalSQL) existing(userID, sum int64) {
	q.mock.ExpectQuery("SELECT user_id, sum FROM public.gophermart_withdrawals WHERE order_number = \\$1").
		WithArgs("18").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "sum"}).AddRow(userID, sum))
}

func (q *withdrawalSQL) absent() {
	q.mock.ExpectQuery("SELECT user_id, sum FROM public.gophermart_withdrawals WHERE order_number = \\$1").
		WithArgs("18").
		WillReturnError(sql.ErrNoRows)
}

func (q *withdrawalSQL) insert() *sqlmock.ExpectedQuery {
	return q.mock.ExpectQuery(`INSERT INTO public.gophermart_withdrawals .* ON CONFLICT \(order_number\) DO NOTHING RETURNING id`).
		WithArgs(int64(7), "18", int64(3000))
}

func (q *withdrawalSQL) update() *sqlmock.ExpectedExec {
	return q.mock.ExpectExec(`UPDATE public.gophermart_balances SET current = current - \$2, withdrawn = withdrawn \+ \$2 WHERE user_id = \$1`).
		WithArgs(int64(7), int64(3000))
}

func (q *withdrawalSQL) inserted() {
	q.balance(10000, 0)
	q.absent()
	q.insert().WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
}

func (q *withdrawalSQL) newDebit(current, withdrawn int64) {
	q.balance(current, withdrawn)
	q.absent()
	q.insert().WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	q.update().WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestWithdrawRetries(t *testing.T) {
	for name, tc := range map[string]struct {
		code         string
		allowUnknown bool
		retry        bool
	}{
		"serialization rollback": {
			code:  "40001",
			retry: true,
		},
		"deadlock rollback": {
			code:  "40P01",
			retry: true,
		},
		"unknown commit rejected by original policy": {
			code: "08006",
		},
		"unknown committed debit safely replayed by explicit policy": {
			code:         "08006",
			allowUnknown: true,
			retry:        true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			failure := &pgconn.PgError{Code: tc.code}
			q := &withdrawalSQL{mock}
			if tc.code == "08006" {
				q.newDebit(10000, 0)
				mock.ExpectCommit().WillReturnError(failure)
			} else {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT current, withdrawn .*FOR UPDATE").WillReturnError(failure)
				mock.ExpectRollback()
			}

			if tc.retry {
				q.balance(0, 3000)
				q.existing(7, 3000)
				mock.ExpectRollback()
			}

			codes := []string{"40001", "40P01"}
			if tc.allowUnknown {
				codes = append(codes, "08006")
			}

			policy, err := strategy.NewExponential(5, 0, time.Second, strategy.NewSQLState(codes))
			require.NoError(t, err)
			err = postgres.NewDBStorage(db, retry.NewRetryer(policy)).
				Withdraw(context.Background(), 7, "18", 3000)
			if tc.retry {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, failure)

				var unknown *postgres.OutcomeUnknownError
				require.ErrorAs(t, err, &unknown)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestWithdrawRejectsAmountAndCanceledContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	storage := postgres.NewDBStorage(db, singleAttempt(t))
	require.ErrorIs(t, storage.Withdraw(context.Background(), 7, "18", 0), model.ErrInvalidWithdrawalAmount)
	require.ErrorIs(t, storage.Withdraw(context.Background(), 7, "18", -1), model.ErrInvalidWithdrawalAmount)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, storage.Withdraw(ctx, 7, "18", 1), context.Canceled)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListWithdrawalsSQL(t *testing.T) {
	failure := errors.New("read failure")
	cleanup := errors.New("close failure")
	stamp := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)

	for name, tc := range map[string]struct {
		rows     *sqlmock.Rows
		queryErr error
		wantErr  error
		scanErr  bool
		closeErr bool
		count    int
	}{
		"ordered history": {
			rows: sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}).
				AddRow(2, 7, "18", 3000, stamp).
				AddRow(1, 7, "0", 1234, stamp),
			count: 2,
		},
		"empty": {
			rows: sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}),
		},
		"query failure": {
			queryErr: failure,
			wantErr:  failure,
		},
		"scan and cleanup failures discard partial result": {
			rows: sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}).
				AddRow(2, 7, "18", 3000, stamp).
				AddRow(1, 7, "0", "invalid", stamp).
				AddRow(0, 7, "26", 1, stamp).
				CloseError(cleanup),
			scanErr:  true,
			closeErr: true,
		},
		"iteration failure discards partial result": {
			rows: sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}).
				AddRow(2, 7, "18", 3000, stamp).
				AddRow(1, 7, "0", 1234, stamp).
				RowError(1, failure),
			wantErr: failure,
		},
		"automatic close failure": {
			rows: sqlmock.NewRows([]string{"id", "user_id", "order_number", "sum", "processed_at"}).
				CloseError(cleanup),
			wantErr: cleanup,
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			query := mock.ExpectQuery(`SELECT id, user_id, order_number, sum, processed_at FROM public.gophermart_withdrawals WHERE user_id = \$1 ORDER BY processed_at DESC, id DESC`).
				WithArgs(int64(7))
			if tc.queryErr != nil {
				query.WillReturnError(tc.queryErr)
			} else {
				query.WillReturnRows(tc.rows).RowsWillBeClosed()
			}

			got, err := postgres.NewDBStorage(db, singleAttempt(t)).
				ListWithdrawalsByUser(context.Background(), 7)
			if tc.scanErr {
				require.Error(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}

			if tc.closeErr {
				require.ErrorIs(t, err, cleanup)
			}

			if err != nil {
				require.Nil(t, got)
			} else {
				require.Len(t, got, tc.count)
				if tc.count > 0 {
					require.Equal(
						t,
						model.Withdrawal{ID: 2, UserID: 7, OrderNumber: "18", Sum: 3000, ProcessedAt: stamp},
						got[0],
					)
					require.Equal(t, int64(1), got[1].ID)
				}
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListWithdrawalsRetryDiscardsPartialRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	stamp := time.Now()
	columns := []string{"id", "user_id", "order_number", "sum", "processed_at"}
	mock.ExpectQuery("SELECT .* FROM public.gophermart_withdrawals").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(2, 7, "18", 3000, stamp).AddRow(1, 7, "0", 1, stamp).
			RowError(1, &pgconn.PgError{Code: "40001"})).
		RowsWillBeClosed()
	mock.ExpectQuery("SELECT .* FROM public.gophermart_withdrawals").WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(2, 7, "18", 3000, stamp)).
		RowsWillBeClosed()
	got, err := postgres.NewDBStorage(db, retryPolicy(t, 5)).ListWithdrawalsByUser(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int64(2), got[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

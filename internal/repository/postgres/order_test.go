package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
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

func TestDBStorageAddOrder(t *testing.T) {
	sqlErr := errors.New("SQL failed")
	fk := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "gophermart_orders_user_fk",
	}

	otherFK := &pgconn.PgError{
		Code:           "23503",
		ConstraintName: "another_fk",
	}

	otherCode := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "gophermart_orders_user_fk",
	}

	tests := map[string]struct {
		insertErr   error
		insertID    any
		conflict    bool
		ownerID     any
		selectErr   error
		missing     bool
		wantCreated bool
		wantErr     error
		scanErr     bool
	}{
		"created": {
			insertID:    int64(42),
			wantCreated: true,
		},
		"own repeat": {
			conflict: true,
			ownerID:  int64(7),
		},
		"foreign repeat": {
			conflict: true,
			ownerID:  int64(8),
			wantErr:  model.ErrOrderOwnedByAnotherUser,
		},
		"insert failure": {
			insertErr: sqlErr,
			wantErr:   sqlErr,
		},
		"deleted user": {
			insertErr: fmt.Errorf("wrapped: %w", fk),
			wantErr:   model.ErrUserNotFound,
		},
		"other foreign key": {
			insertErr: otherFK,
			wantErr:   otherFK,
		},
		"other SQLSTATE": {
			insertErr: otherCode,
			wantErr:   otherCode,
		},
		"select failure": {
			conflict:  true,
			selectErr: sqlErr,
			wantErr:   sqlErr,
		},
		"disappeared": {
			conflict: true,
			missing:  true,
			wantErr:  sql.ErrNoRows,
		},
		"insert scan failure": {
			insertID: "bad ID",
			scanErr:  true,
		},
		"owner scan failure": {
			conflict: true,
			ownerID:  "bad ID",
			scanErr:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			insert := mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO public.gophermart_orders (user_id, number) VALUES ($1, $2) ON CONFLICT (number) DO NOTHING RETURNING id`)).
				WithArgs(int64(7), "18")

			switch {
			case tc.insertErr != nil:
				insert.WillReturnError(tc.insertErr)
			case tc.conflict:
				insert.WillReturnRows(sqlmock.NewRows([]string{"id"}))
				query := mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id FROM public.gophermart_orders WHERE number = $1`)).
					WithArgs("18")

				switch {
				case tc.selectErr != nil:
					query.WillReturnError(tc.selectErr)
				case tc.missing:
					query.WillReturnRows(sqlmock.NewRows([]string{"user_id"}))
				default:
					query.WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(tc.ownerID))
				}
			default:
				insert.WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(tc.insertID))
			}

			created, err := postgres.NewDBStorage(db, singleAttempt(t)).
				AddOrder(context.Background(), 7, "18")
			require.Equal(t, tc.wantCreated, created)
			if tc.scanErr {
				require.Error(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDBStorageListOrdersByUser(t *testing.T) {
	stamp := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	sqlErr := errors.New("query or rows failed")
	columns := []string{"id", "user_id", "number", "status", "accrual", "uploaded_at"}
	tests := map[string]struct {
		rows     *sqlmock.Rows
		queryErr error
		wantErr  error
		scanErr  bool
		count    int
	}{
		"orders": {
			rows: sqlmock.NewRows(columns).
				AddRow(4, 7, "18", "NEW", nil, stamp).
				AddRow(3, 7, "0", "INVALID", 0, stamp).
				AddRow(2, 7, "26", "PROCESSED", 12345, stamp).
				AddRow(1, 7, "34", "PROCESSING", nil, stamp),
			count: 4,
		},
		"empty": {
			rows: sqlmock.NewRows(columns),
		},
		"query failure": {
			queryErr: sqlErr,
			wantErr:  sqlErr,
		},
		"scan failure after first row": {
			rows: sqlmock.NewRows(columns).
				AddRow(2, 7, "18", "NEW", nil, stamp).
				AddRow("bad", 7, "0", "NEW", nil, stamp),
			scanErr: true,
		},
		"iteration failure after first row": {
			rows: sqlmock.NewRows(columns).
				AddRow(2, 7, "18", "NEW", nil, stamp).
				AddRow(1, 7, "0", "NEW", nil, stamp).RowError(1, sqlErr),
			wantErr: sqlErr,
		},
		"close failure": {
			rows:    sqlmock.NewRows(columns).CloseError(sqlErr),
			wantErr: sqlErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			query := mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, number, status, accrual, uploaded_at FROM public.gophermart_orders WHERE user_id = $1 ORDER BY uploaded_at DESC, id DESC`)).
				WithArgs(int64(7))
			if tc.queryErr != nil {
				query.WillReturnError(tc.queryErr)
			} else {
				query.WillReturnRows(tc.rows).RowsWillBeClosed()
			}

			orders, err := postgres.NewDBStorage(db, singleAttempt(t)).
				ListOrdersByUser(context.Background(), 7)
			if tc.scanErr {
				require.Error(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}

			if tc.scanErr || tc.wantErr != nil {
				require.Nil(t, orders)
			}

			require.Len(t, orders, tc.count)
			if tc.count > 0 {
				require.Equal(
					t,
					[]string{"18", "0", "26", "34"},
					[]string{orders[0].Number, orders[1].Number, orders[2].Number, orders[3].Number},
				)
				require.Nil(t, orders[0].Accrual)
				require.Nil(t, orders[3].Accrual)
				require.Equal(t, int64(0), *orders[1].Accrual)
				require.Equal(t, int64(12345), *orders[2].Accrual)
				require.Equal(t, model.OrderStatusProcessed, orders[2].Status)
				require.Equal(t, stamp, orders[0].UploadedAt)
				require.Equal(t, int64(7), orders[0].UserID)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestListPendingRetryDiscardsPartialRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	failure := &pgconn.PgError{Code: "40001"}
	query := regexp.QuoteMeta(`SELECT id, user_id, number, status, accrual, uploaded_at FROM public.gophermart_orders WHERE status IN ('NEW', 'PROCESSING') ORDER BY id ASC`)
	columns := []string{"id", "user_id", "number", "status", "accrual", "uploaded_at"}
	now := time.Now()
	first := sqlmock.NewRows(columns).AddRow(1, 7, "18", "NEW", nil, now).
		AddRow(2, 7, "26", "PROCESSING", nil, now).
		RowError(1, failure)
	mock.ExpectQuery(query).WillReturnRows(first).RowsWillBeClosed()
	mock.ExpectQuery(query).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(2, 7, "26", "PROCESSING", nil, now)).
		RowsWillBeClosed()
	orders, err := postgres.NewDBStorage(db, retryPolicy(t, 5)).ListPending(context.Background())
	require.NoError(t, err)
	require.Len(t, orders, 1)
	require.Equal(t, int64(2), orders[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestListPendingEmpty(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("WHERE status IN .* ORDER BY id ASC").
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"number",
			"status",
			"accrual",
			"uploaded_at",
		})).
		RowsWillBeClosed()
	orders, err := postgres.NewDBStorage(db, singleAttempt(t)).ListPending(context.Background())
	require.NoError(t, err)
	require.Empty(t, orders)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAddOrderRetryPreservesCreation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery("INSERT INTO public.gophermart_orders").WillReturnError(&pgconn.PgError{Code: "40001"})
	mock.ExpectQuery("INSERT INTO public.gophermart_orders").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	created, err := postgres.NewDBStorage(db, retryPolicy(t, 5)).AddOrder(context.Background(), 7, "18")
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAddOrderUnknownOutcomeCannotReplayEvenWhenMatched(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	failure := &pgconn.PgError{Code: "08006", Message: "lost acknowledgement"}
	mock.ExpectQuery("INSERT INTO public.gophermart_orders").WillReturnError(failure)
	policy, err := strategy.NewExponential(5, 0, time.Second, strategy.NewSQLState([]string{"08006"}))
	require.NoError(t, err)
	created, err := postgres.NewDBStorage(db, retry.NewRetryer(policy)).
		AddOrder(context.Background(), 7, "18")
	require.False(t, created)
	require.ErrorIs(t, err, failure)

	var permanent *retry.PermanentError
	require.ErrorAs(t, err, &permanent)
	require.NoError(t, mock.ExpectationsWereMet())
}

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/repository/postgres"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

const (
	insertUserSQL    = `INSERT INTO public.gophermart_users (login, password_hash) VALUES ($1, $2) RETURNING id, login, password_hash`
	insertBalanceSQL = `INSERT INTO public.gophermart_balances (user_id, current, withdrawn) VALUES ($1, 0, 0)`
	selectUserSQL    = `SELECT id, login, password_hash FROM public.gophermart_users WHERE login = $1`
)

func TestDBStorageAddUser(t *testing.T) {
	sqlErr := errors.New("database failure")
	rollbackErr := errors.New("rollback failure")
	conflict := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "gophermart_users_login_key",
	}
	otherConstraint := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "gophermart_users_pkey",
	}
	otherCode := &pgconn.PgError{
		Code:           "23514",
		ConstraintName: "gophermart_users_login_key",
	}
	tests := map[string]struct {
		beginErr    error
		insertErr   error
		scanError   bool
		balanceErr  error
		commitErr   error
		rollbackErr error
		wantErr     error
	}{
		"success": {},
		"begin failure": {
			beginErr: sqlErr,
			wantErr:  sqlErr,
		},
		"login conflict": {
			insertErr: fmt.Errorf("driver: %w", conflict),
			wantErr:   model.ErrLoginTaken,
		},
		"other constraint": {
			insertErr: otherConstraint,
			wantErr:   otherConstraint,
		},
		"other SQLSTATE": {
			insertErr: otherCode,
			wantErr:   otherCode,
		},
		"insert failure": {
			insertErr: sqlErr,
			wantErr:   sqlErr,
		},
		"scan failure": {
			scanError: true,
		},
		"balance failure": {
			balanceErr: sqlErr,
			wantErr:    sqlErr,
		},
		"commit failure": {
			commitErr: sqlErr,
			wantErr:   sqlErr,
		},
		"rollback failure": {
			balanceErr:  sqlErr,
			rollbackErr: rollbackErr,
			wantErr:     sqlErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			begin := mock.ExpectBegin()
			if tc.beginErr != nil {
				begin.WillReturnError(tc.beginErr)
			}
			if tc.beginErr == nil {
				insert := mock.ExpectQuery(regexp.QuoteMeta(insertUserSQL)).WithArgs("Alice", "hash")
				switch {
				case tc.insertErr != nil:
					insert.WillReturnError(tc.insertErr)
				case tc.scanError:
					insert.WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password_hash"}).AddRow("bad id", "Alice", "hash"))
				default:
					insert.WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password_hash"}).AddRow(42, "Alice", "hash"))
				}

				if tc.insertErr == nil && !tc.scanError {
					balance := mock.ExpectExec(regexp.QuoteMeta(insertBalanceSQL)).WithArgs(int64(42))
					if tc.balanceErr != nil {
						balance.WillReturnError(tc.balanceErr)
					} else {
						balance.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}

				if tc.insertErr != nil || tc.scanError || tc.balanceErr != nil {
					rollback := mock.ExpectRollback()
					if tc.rollbackErr != nil {
						rollback.WillReturnError(tc.rollbackErr)
					}
				} else {
					commit := mock.ExpectCommit()
					if tc.commitErr != nil {
						commit.WillReturnError(tc.commitErr)
					}
					// database/sql marks the transaction done even when Commit returns an error;
					// the deferred Rollback returns ErrTxDone without calling the driver again.
				}
			}

			user, err := postgres.NewDBStorage(db).AddUser(context.Background(), "Alice", "hash")
			if tc.wantErr != nil || tc.scanError {
				require.Error(t, err)
				require.Equal(t, model.User{}, user)
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr)
				}
				if tc.rollbackErr != nil {
					require.ErrorIs(t, err, tc.rollbackErr)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, model.User{ID: 42, Login: "Alice", PasswordHash: "hash"}, user)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDBStorageFindUserByLogin(t *testing.T) {
	sqlErr := errors.New("select failure")
	tests := map[string]struct {
		rows      *sqlmock.Rows
		queryErr  error
		wantErr   error
		scanError bool
	}{
		"found": {
			rows: sqlmock.NewRows([]string{"id", "login", "password_hash"}).AddRow(42, " Alice ", "hash"),
		},
		"absent": {
			rows:    sqlmock.NewRows([]string{"id", "login", "password_hash"}),
			wantErr: model.ErrUserNotFound,
		},
		"query failure": {
			queryErr: sqlErr,
			wantErr:  sqlErr,
		},
		"scan failure": {
			rows:      sqlmock.NewRows([]string{"id", "login", "password_hash"}).AddRow(nil, " Alice ", "hash"),
			scanError: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			query := mock.ExpectQuery(regexp.QuoteMeta(selectUserSQL)).WithArgs(" Alice ")
			if tc.queryErr != nil {
				query.WillReturnError(tc.queryErr)
			} else {
				query.WillReturnRows(tc.rows)
			}

			user, err := postgres.NewDBStorage(db).FindUserByLogin(context.Background(), " Alice ")
			if tc.wantErr != nil || tc.scanError {
				require.Error(t, err)
				require.Equal(t, model.User{}, user)
				if tc.wantErr != nil {
					require.ErrorIs(t, err, tc.wantErr)
				}
			} else {
				require.NoError(t, err)
				require.Equal(t, model.User{ID: 42, Login: " Alice ", PasswordHash: "hash"}, user)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDBStorageUserExists(t *testing.T) {
	queryErr := errors.New("query failed")
	tests := map[string]struct {
		value    any
		queryErr error
		want     bool
		wantErr  bool
	}{
		"present": {
			value: true,
			want:  true,
		},
		"missing": {
			value: false,
		},
		"query failure": {
			queryErr: queryErr,
			wantErr:  true,
		},
		"scan failure": {
			value:   "invalid boolean",
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			query := mock.ExpectQuery(regexp.QuoteMeta(`SELECT EXISTS (SELECT 1 FROM public.gophermart_users WHERE id = $1)`)).WithArgs(int64(7))
			if tc.queryErr != nil {
				query.WillReturnError(tc.queryErr)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tc.value))
			}

			got, err := postgres.NewDBStorage(db).UserExists(context.Background(), 7)
			require.Equal(t, tc.want, got)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tc.queryErr != nil {
				require.ErrorIs(t, err, tc.queryErr)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

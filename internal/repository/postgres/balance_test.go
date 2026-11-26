package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/repository/postgres"
	"github.com/stretchr/testify/require"
)

func TestGetBalance(t *testing.T) {
	for name, tc := range map[string]struct {
		missing bool
		scan    bool
		err     error
	}{
		"found": {},
		"missing": {
			missing: true,
		},
		"scan error": {
			scan: true,
		},
		"query error": {
			err: errors.New("query failed"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			rows := sqlmock.NewRows([]string{"current", "withdrawn"})
			if !tc.missing {
				rows.AddRow(123, 456)
			}

			if tc.scan {
				rows = sqlmock.NewRows([]string{"current", "withdrawn"}).AddRow(123, "invalid")
			}

			mock.ExpectQuery("SELECT current, withdrawn FROM public.gophermart_balances").WithArgs(int64(7)).
				WillReturnRows(rows).
				WillReturnError(tc.err)
			got, err := postgres.NewDBStorage(db, singleAttempt(t)).GetBalance(context.Background(), 7)

			switch {
			case tc.missing:
				require.ErrorIs(t, err, model.ErrBalanceNotFound)
			case tc.scan || tc.err != nil:
				require.Error(t, err)
				require.Zero(t, got)
			default:
				require.NoError(t, err)
				require.Equal(t, model.Balance{Current: 123, Withdrawn: 456}, got)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

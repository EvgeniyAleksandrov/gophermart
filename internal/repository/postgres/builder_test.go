package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EvgeniyAleksandrov/gophermart/migrations"
	"github.com/stretchr/testify/require"
)

func TestDBStorageBuilderInitialize(t *testing.T) {
	failure := errors.New("ping failed")
	testCases := map[string]struct {
		pingErr  error
		canceled bool
	}{
		"success": {},
		"ping failure": {
			pingErr: failure,
		},
		"canceled": {
			canceled: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.canceled {
				cancel()
			} else {
				mock.ExpectPing().WillReturnError(tc.pingErr)
			}

			builder := NewDBStorageBuilder("unused", nil)
			err = builder.initialize(ctx, db)
			switch {
			case tc.canceled:
				require.ErrorIs(t, err, context.Canceled)
			case tc.pingErr != nil:
				require.ErrorIs(t, err, tc.pingErr)
			default:
				require.NoError(t, err)
			}
			require.Equal(t, maxOpenConnections, db.Stats().MaxOpenConnections)

			mock.ExpectClose()
			require.NoError(t, db.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDBStorageBuilderFailure(t *testing.T) {
	builder := NewDBStorageBuilder("postgres://user:secret@localhost:invalid/db", nil)
	storage, err := builder.Build(context.Background())
	require.Error(t, err)
	require.Nil(t, storage)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	builder = NewDBStorageBuilder("host=localhost user=test dbname=test sslmode=disable", nil)
	storage, err = builder.Build(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, storage)
}

func TestDBStorageClose(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	storage := NewDBStorage(db)
	mock.ExpectClose()

	require.NoError(t, storage.Close())
	require.NoError(t, storage.Close())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBStorageBuilderClosesPoolOnPingFailure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	failure := errors.New("ping failed")
	mock.ExpectPing().WillReturnError(failure)
	mock.ExpectClose()

	storage, err := NewDBStorageBuilder("unused", nil).prepare(context.Background(), db)
	require.ErrorIs(t, err, failure)
	require.Nil(t, storage)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBStorageBuilderPrepare(t *testing.T) {
	testCases := map[string]struct {
		dirty bool
	}{
		"ready storage": {},
		"migration failure closes pool": {
			dirty: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := &migrationFixture{}
			fixture.setup(t)
			fixture.mock.ExpectPing()
			fixture.settings()
			fixture.initialize()
			fixture.version(4, tc.dirty)
			fixture.mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			fixture.restore(nil)
			if tc.dirty {
				fixture.mock.ExpectClose()
			}

			storage, err := NewDBStorageBuilder("unused", migrations.NewSource().FS()).prepare(context.Background(), fixture.db)
			if tc.dirty {
				require.Error(t, err)
				require.Nil(t, storage)
				require.ErrorContains(t, fixture.db.Ping(), "database is closed")
			} else {
				require.NoError(t, err)
				require.Same(t, fixture.db, storage.db)
				fixture.mock.ExpectPing()
				require.NoError(t, storage.db.Ping())
				fixture.mock.ExpectClose()
				require.NoError(t, storage.Close())
			}

			require.NoError(t, fixture.mock.ExpectationsWereMet())
		})
	}
}

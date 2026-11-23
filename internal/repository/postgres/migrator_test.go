package postgres_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/repository/postgres"
	"github.com/EvgeniyAleksandrov/gophermart/migrations"
	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/require"
)

const settingsSQL = "SELECT set_config('statement_timeout', $1, false), set_config('lock_timeout', $2, false)"

type migrationFixture struct {
	db   *sql.DB
	mock sqlmock.Sqlmock
}

func (f *migrationFixture) setup(t *testing.T) {
	t.Helper()
	var err error
	f.db, f.mock, err = sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { f.db.Close() })
}

func (f *migrationFixture) settings() {
	f.mock.ExpectQuery(regexp.QuoteMeta("SELECT current_setting('statement_timeout'), current_setting('lock_timeout')")).
		WillReturnRows(sqlmock.NewRows([]string{"statement", "lock"}).AddRow("2min", "1s"))
	f.mock.ExpectExec(regexp.QuoteMeta(settingsSQL)).WithArgs("30000ms", "5000ms").WillReturnResult(sqlmock.NewResult(0, 1))
}

func (f *migrationFixture) initialize() {
	f.mock.ExpectPing()
	f.mock.ExpectQuery("SELECT CURRENT_DATABASE").WillReturnRows(sqlmock.NewRows([]string{"database"}).AddRow("test"))
	f.mock.ExpectExec("SELECT pg_advisory_lock").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectQuery("SELECT COUNT").WithArgs("public", "gophermart_schema_migrations").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	f.mock.ExpectExec(`CREATE TABLE IF NOT EXISTS "public"."gophermart_schema_migrations"`).WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
}

func (f *migrationFixture) version(version int, dirty bool) {
	f.mock.ExpectExec("SELECT pg_advisory_lock").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	rows := sqlmock.NewRows([]string{"version", "dirty"})
	if version >= 0 {
		rows.AddRow(version, dirty)
	}
	f.mock.ExpectQuery(`SELECT version, dirty FROM "public"."gophermart_schema_migrations" LIMIT 1`).WillReturnRows(rows)
}

func (f *migrationFixture) setVersion(version int, dirty bool) {
	f.mock.ExpectBegin()
	f.mock.ExpectExec(`TRUNCATE "public"."gophermart_schema_migrations"`).WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectExec(`INSERT INTO "public"."gophermart_schema_migrations"`).WithArgs(version, dirty).WillReturnResult(sqlmock.NewResult(0, 1))
	f.mock.ExpectCommit()
}

func (f *migrationFixture) restore(err error) {
	f.mock.ExpectExec(regexp.QuoteMeta(settingsSQL)).WithArgs("2min", "1s").WillReturnResult(sqlmock.NewResult(0, 1)).WillReturnError(err)
}

func TestMigratorUp(t *testing.T) {
	testCases := map[string]struct {
		version      int
		dirty        bool
		ddlError     bool
		restoreError bool
	}{
		"new database": {
			version: -1,
		},
		"no change": {
			version: 5,
		},
		"dirty": {
			version: 2,
			dirty:   true,
		},
		"DDL failure": {
			version:  -1,
			ddlError: true,
		},
		"restore failure": {
			version:      5,
			restoreError: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := &migrationFixture{}
			fixture.setup(t)
			fixture.settings()
			fixture.initialize()
			fixture.version(tc.version, tc.dirty)
			failure := errors.New("DDL failed")
			if tc.version < 0 {
				for version, table := range []string{"users", "balances", "orders", "withdrawals"} {
					fixture.setVersion(version+1, true)
					expectation := fixture.mock.ExpectExec("BEGIN;.*CREATE TABLE public.gophermart_" + table)
					if tc.ddlError {
						expectation.WillReturnError(failure)
						break
					}
					expectation.WillReturnResult(sqlmock.NewResult(0, 1))
					fixture.setVersion(version+1, false)
				}
			}

			if tc.version < 0 && !tc.ddlError {
				fixture.setVersion(5, true)
				fixture.mock.ExpectExec("CREATE INDEX gophermart_orders_pending_id_idx").WillReturnResult(sqlmock.NewResult(0, 1))
				fixture.setVersion(5, false)
			}
			fixture.mock.ExpectExec("SELECT pg_advisory_unlock").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
			var restoreErr error
			if tc.restoreError {
				restoreErr = failure
			}
			fixture.restore(restoreErr)
			failed := tc.dirty || tc.ddlError || tc.restoreError
			if failed {
				fixture.mock.ExpectClose()
			}

			err := postgres.NewMigrator(fixture.db, migrations.NewSource().FS()).Up(context.Background())
			if failed {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				fixture.mock.ExpectPing()
				require.NoError(t, fixture.db.Ping())
			}

			if tc.dirty {
				var dirty migrate.ErrDirty
				require.ErrorAs(t, err, &dirty)
				require.Equal(t, 2, dirty.Version)
			}
			require.NoError(t, fixture.mock.ExpectationsWereMet())
		})
	}
}

func TestMigratorInitializationFailure(t *testing.T) {
	fixture := &migrationFixture{}
	fixture.setup(t)
	fixture.settings()
	failure := errors.New("ping failed")
	fixture.mock.ExpectPing().WillReturnError(failure)
	fixture.restore(nil)
	fixture.mock.ExpectClose()

	err := postgres.NewMigrator(fixture.db, migrations.NewSource().FS()).Up(context.Background())
	require.ErrorIs(t, err, failure)
	require.NoError(t, fixture.mock.ExpectationsWereMet())
}

func TestMigratorSourceAndCancellation(t *testing.T) {
	fixture := &migrationFixture{}
	fixture.setup(t)
	require.Error(t, postgres.NewMigrator(fixture.db, fstest.MapFS{}).Up(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, postgres.NewMigrator(fixture.db, migrations.NewSource().FS()).Up(ctx), context.Canceled)
	require.NoError(t, fixture.mock.ExpectationsWereMet())
}

func TestMigratorTimeoutSetupFailure(t *testing.T) {
	failure := errors.New("timeout setup failure")
	testCases := map[string]struct {
		readFailure bool
	}{
		"read failure": {
			readFailure: true,
		},
		"set failure restores saved values": {},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			fixture := &migrationFixture{}
			fixture.setup(t)
			read := fixture.mock.ExpectQuery(regexp.QuoteMeta("SELECT current_setting('statement_timeout'), current_setting('lock_timeout')"))
			if tc.readFailure {
				read.WillReturnError(failure)
			} else {
				read.WillReturnRows(sqlmock.NewRows([]string{"statement", "lock"}).AddRow("2min", "1s"))
				fixture.mock.ExpectExec(regexp.QuoteMeta(settingsSQL)).WithArgs("30000ms", "5000ms").WillReturnError(failure)
				fixture.restore(nil)
			}
			fixture.mock.ExpectClose()

			err := postgres.NewMigrator(fixture.db, migrations.NewSource().FS()).Up(context.Background())
			require.ErrorIs(t, err, failure)
			require.Zero(t, fixture.db.Stats().InUse)
			require.NoError(t, fixture.mock.ExpectationsWereMet())
		})
	}
}

// cancelOnArgument cancels the application context during the adapter's final
// unlock, before Up returns. Cleanup must still restore the session settings.
type cancelOnArgument struct {
	cancel context.CancelFunc
}

func (a cancelOnArgument) Match(driver.Value) bool {
	a.cancel()
	return true
}

func TestMigratorCancellationDuringUp(t *testing.T) {
	fixture := &migrationFixture{}
	fixture.setup(t)
	fixture.settings()
	fixture.initialize()
	fixture.version(5, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fixture.mock.ExpectExec("SELECT pg_advisory_unlock").
		WithArgs(cancelOnArgument{cancel: cancel}).WillReturnResult(sqlmock.NewResult(0, 1))
	fixture.restore(nil)
	fixture.mock.ExpectClose()

	err := postgres.NewMigrator(fixture.db, migrations.NewSource().FS()).Up(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, fixture.db.Stats().InUse)
	require.NoError(t, fixture.mock.ExpectationsWereMet())
}

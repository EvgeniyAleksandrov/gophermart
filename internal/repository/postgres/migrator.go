package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const (
	migrationsSchema          = "public"
	migrationsTable           = "gophermart_schema_migrations"
	migrationStatementTimeout = 30 * time.Second
	migrationLockTimeout      = 5 * time.Second
)

// Migrator applies embedded SQL on a dedicated connection without owning the pool.
type Migrator struct {
	db    *sql.DB
	files fs.FS
}

// NewMigrator receives an open pool and a versioned SQL source.
func NewMigrator(db *sql.DB, files fs.FS) *Migrator {
	return &Migrator{db: db, files: files}
}

// Up applies all pending migrations, treating ErrNoChange as success. It never
// resets dirty state or retries. During migrate.Up cancellation waits for SQL
// bounded by server timeouts; the library does not accept a context for Up.
func (m *Migrator) Up(ctx context.Context) (err error) {
	source, err := iofs.New(m.files, ".")
	if err != nil {
		return fmt.Errorf("open migration source: %w", err)
	}

	// Until migrate takes ownership, this cleanup also handles partial setup.
	var conn *sql.Conn
	var runner *migrate.Migrate
	var settings *migrationSettings
	defer func() {
		if settings != nil {
			err = errors.Join(err, settings.restore(conn))
		}
		// Failed SQL may leave a transaction or advisory lock behind. Discard the
		// physical session on failure, including failed restoration, before Close.
		if conn != nil && err != nil {
			discardErr := conn.Raw(func(any) error { return driver.ErrBadConn })
			if !errors.Is(discardErr, driver.ErrBadConn) && !errors.Is(discardErr, sql.ErrConnDone) {
				err = errors.Join(err, discardErr)
			}
		}
		if runner != nil {
			sourceErr, databaseErr := runner.Close()
			err = errors.Join(err, sourceErr, databaseErr)
			return
		}

		err = errors.Join(err, source.Close())
		if conn != nil {
			err = errors.Join(err, conn.Close())
		}
	}()

	initCtx, cancel := context.WithTimeout(ctx, connectionPingTimeout)
	defer cancel()

	conn, err = m.db.Conn(initCtx)
	if err != nil {
		return fmt.Errorf("acquire migration connection: %w", err)
	}

	settings = &migrationSettings{}
	if err := settings.apply(initCtx, conn); err != nil {
		return err
	}

	adapter, err := migratepostgres.WithConnection(initCtx, conn, &migratepostgres.Config{
		SchemaName:      migrationsSchema,
		MigrationsTable: migrationsTable,
	})
	if err != nil {
		return fmt.Errorf("initialize migration driver: %w", err)
	}

	runner, err = migrate.NewWithInstance("iofs", source, "postgres", adapter)
	if err != nil {
		return fmt.Errorf("initialize migrator: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	err = runner.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		err = nil
	}
	if err != nil {
		return err
	}

	return ctx.Err()
}

type migrationSettings struct {
	statement string
	lock      string
	saved     bool
}

func (s *migrationSettings) apply(ctx context.Context, conn *sql.Conn) error {
	err := conn.QueryRowContext(ctx,
		"SELECT current_setting('statement_timeout'), current_setting('lock_timeout')",
	).Scan(&s.statement, &s.lock)
	if err != nil {
		return fmt.Errorf("read migration timeouts: %w", err)
	}
	s.saved = true

	_, err = conn.ExecContext(ctx,
		"SELECT set_config('statement_timeout', $1, false), set_config('lock_timeout', $2, false)",
		fmt.Sprintf("%dms", migrationStatementTimeout.Milliseconds()),
		fmt.Sprintf("%dms", migrationLockTimeout.Milliseconds()),
	)
	if err != nil {
		return fmt.Errorf("set migration timeouts: %w", err)
	}

	return nil
}

func (s *migrationSettings) restore(conn *sql.Conn) error {
	if !s.saved {
		return nil
	}

	// Cleanup must work even when the application's context has been canceled.
	ctx, cancel := context.WithTimeout(context.Background(), connectionPingTimeout)
	defer cancel()

	_, err := conn.ExecContext(ctx,
		"SELECT set_config('statement_timeout', $1, false), set_config('lock_timeout', $2, false)",
		s.statement, s.lock,
	)
	if err != nil {
		return fmt.Errorf("restore migration timeouts: %w", err)
	}

	return nil
}

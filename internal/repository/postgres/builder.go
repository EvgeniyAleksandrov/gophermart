package postgres

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const (
	maxOpenConnections    = 10
	maxIdleConnections    = 5
	connectionMaxLifetime = 30 * time.Minute
	connectionMaxIdleTime = 5 * time.Minute
	connectionPingTimeout = 5 * time.Second
)

type DBStorageBuilder struct {
	dsn     string
	files   fs.FS
	retryer interfaces.Retryer
}

func NewDBStorageBuilder(dsn string, files fs.FS, retryer interfaces.Retryer) *DBStorageBuilder {
	return &DBStorageBuilder{dsn: dsn, files: files, retryer: retryer}
}

func (b *DBStorageBuilder) Build(ctx context.Context) (*DBStorage, error) {
	cfg, err := pgx.ParseConfig(b.dsn)
	if err != nil {
		return nil, err
	}

	return b.prepare(ctx, stdlib.OpenDB(*cfg))
}

func (b *DBStorageBuilder) prepare(ctx context.Context, db *sql.DB) (*DBStorage, error) {
	if err := b.initialize(ctx, db); err != nil {
		return nil, errors.Join(err, db.Close())
	}

	if err := NewMigrator(db, b.files).Up(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}

	return NewDBStorage(db, b.retryer), nil
}

func (b *DBStorageBuilder) initialize(ctx context.Context, db *sql.DB) error {
	db.SetMaxOpenConns(maxOpenConnections)
	db.SetMaxIdleConns(maxIdleConnections)
	db.SetConnMaxLifetime(connectionMaxLifetime)
	db.SetConnMaxIdleTime(connectionMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, connectionPingTimeout)
	defer cancel()

	return db.PingContext(pingCtx)
}

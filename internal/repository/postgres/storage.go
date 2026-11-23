package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
)

// DBStorage owns a prepared SQL pool and exposes atomic database operations.
// Consumers use its methods rather than accessing the pool directly.
type DBStorage struct {
	db      *sql.DB
	retryer interfaces.Retryer
}

// NewDBStorage takes ownership of a prepared pool and receives the shared retryer.
func NewDBStorage(db *sql.DB, retryer interfaces.Retryer) *DBStorage {
	return &DBStorage{db: db, retryer: retryer}
}

// Close releases the pool after all consumers have stopped using storage.
func (s *DBStorage) Close() error { return s.db.Close() }

// operation owns one complete attempt, including transaction/rows cleanup.
type operation interface {
	Execute(context.Context, *sql.DB) error
	CanReplayUnknownOutcome() bool
}

func (s *DBStorage) execute(ctx context.Context, op operation) error {
	return s.retryer.Do(ctx, func() error {
		err := op.Execute(ctx, s.db)
		var unknown *OutcomeUnknownError
		if errors.As(err, &unknown) && !op.CanReplayUnknownOutcome() {
			return &retry.PermanentError{Err: err}
		}
		return err
	})
}

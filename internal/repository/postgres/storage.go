package postgres

import (
	"database/sql"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
)

// DBStorage owns a prepared SQL pool and exposes atomic database operations.
// Consumers use its methods rather than accessing the pool directly.
type DBStorage struct {
	db      *sql.DB
	retryer interfaces.Retryer
}

func NewDBStorage(db *sql.DB, retryer interfaces.Retryer) *DBStorage {
	return &DBStorage{db: db, retryer: retryer}
}

func (s *DBStorage) Close() error { return s.db.Close() }

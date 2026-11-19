package postgres

import "database/sql"

// DBStorage owns a prepared SQL pool and exposes atomic database operations.
// Consumers use its methods rather than accessing the pool directly.
type DBStorage struct {
	db *sql.DB
}

// NewDBStorage takes ownership of a pool already configured, checked and migrated.
func NewDBStorage(db *sql.DB) *DBStorage {
	return &DBStorage{db: db}
}

// Close releases the pool after all consumers have stopped using storage.
func (s *DBStorage) Close() error {
	return s.db.Close()
}

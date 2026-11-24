// Package postgres owns the SQL pool, embedded schema migration lifecycle and
// atomic storage operations. Startup does not use application retries. Public
// DBStorage methods delegate complete attempts to the injected retryer; private
// methods own SQL and transactions and release resources before another attempt.
// Database handles and transactions never cross into business services.
package postgres

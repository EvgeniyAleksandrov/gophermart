// Package postgres manages the SQL pool and schema migrations. Startup does not
// use application retries. Future repositories own their SQL transactions;
// database handles and transactions must not cross into business services.
package postgres

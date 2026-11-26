package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func (s *DBStorage) classifyWriteOutcome(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(
		err,
		&pgErr,
	) && (strings.HasPrefix(pgErr.Code, "23") || pgErr.Code == "40001" || pgErr.Code == "40P01") {
		return err
	}

	return &OutcomeUnknownError{Err: err}
}

type OutcomeUnknownError struct {
	Err error
}

func (e *OutcomeUnknownError) Error() string { return "unknown write outcome: " + e.Err.Error() }

func (e *OutcomeUnknownError) Unwrap() error { return e.Err }

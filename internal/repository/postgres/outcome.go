package postgres

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// writeOutcome distinguishes a server-reported rejection from an uncertain write.
type writeOutcome struct{}

func (o *writeOutcome) classify(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (strings.HasPrefix(pgErr.Code, "23") || pgErr.Code == "40001" || pgErr.Code == "40P01") {
		return err
	}

	return &OutcomeUnknownError{Err: err}
}

// OutcomeUnknownError marks a write whose durable outcome is not known.
type OutcomeUnknownError struct{ Err error }

// Error describes the uncertain write.
func (e *OutcomeUnknownError) Error() string { return "unknown write outcome: " + e.Err.Error() }

// Unwrap preserves the underlying error for retry classification.
func (e *OutcomeUnknownError) Unwrap() error { return e.Err }

package model

import "time"

// AccrualStatus is a calculation state returned by the external service.
type AccrualStatus string

const (
	// AccrualRegistered means the order awaits calculation.
	AccrualRegistered AccrualStatus = "REGISTERED"
	// AccrualProcessing means calculation is in progress.
	AccrualProcessing AccrualStatus = "PROCESSING"
	// AccrualInvalid is a final rejection.
	AccrualInvalid AccrualStatus = "INVALID"
	// AccrualProcessed is a final calculation.
	AccrualProcessed AccrualStatus = "PROCESSED"
)

// AccrualResult contains a validated external result, with amounts in hundredths.
type AccrualResult struct {
	Number  string
	Status  AccrualStatus
	Accrual int64
}

// RateLimitError carries the validated delay for retrying a rate-limited request.
type RateLimitError struct{ RetryAfter time.Duration }

// Error describes rate limiting without HTTP details.
func (e *RateLimitError) Error() string { return "accrual rate limit" }

// PersistenceError distinguishes storage failures from external service failures.
type PersistenceError struct{ Err error }

// Error describes the failed persistence operation.
func (e *PersistenceError) Error() string { return "persist accrual: " + e.Err.Error() }

// Unwrap preserves the storage error.
func (e *PersistenceError) Unwrap() error { return e.Err }

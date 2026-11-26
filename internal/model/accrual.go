package model

import "time"

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

type AccrualResult struct {
	Number  string
	Status  AccrualStatus
	Accrual int64
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return "accrual rate limit" }

type PersistenceError struct {
	Err error
}

func (e *PersistenceError) Error() string { return "persist accrual: " + e.Err.Error() }

func (e *PersistenceError) Unwrap() error { return e.Err }

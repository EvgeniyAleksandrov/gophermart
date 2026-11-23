package retry

// PermanentError marks an operation failure that must not be replayed, even when
// the underlying error would match a retry strategy.
type PermanentError struct{ Err error }

// Error preserves the diagnostic message of the operation failure.
func (e *PermanentError) Error() string { return e.Err.Error() }

// Unwrap preserves the original error's identity and type.
func (e *PermanentError) Unwrap() error { return e.Err }

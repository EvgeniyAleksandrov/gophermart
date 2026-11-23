package interfaces

import "context"

// Retryer executes a whole operation and returns its final error to the caller.
// The operation releases resources before returning and captures its context.
type Retryer interface {
	Do(context.Context, func() error) error
}

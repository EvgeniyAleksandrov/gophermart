// Package requestcontext provides typed request-scoped context values.
package requestcontext

import "context"

type userContextKey struct{}

// User attaches and reads the authenticated user ID without a string key.
type User struct{}

// NewUser constructs a stateless user context accessor.
func NewUser() *User {
	return &User{}
}

// WithID derives a context while preserving its parent's cancellation and values.
func (c *User) WithID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userContextKey{}, id)
}

// ID returns the attached user ID and whether one is present.
func (c *User) ID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userContextKey{}).(int64)
	return id, ok
}

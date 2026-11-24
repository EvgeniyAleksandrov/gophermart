// Package requestcontext provides typed request-scoped context values.
package requestcontext

import "context"

type userContextKey struct {
}

type User struct {
}

func NewUser() *User {
	return &User{}
}

func (c *User) WithID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, userContextKey{}, id)
}

func (c *User) ID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userContextKey{}).(int64)
	return id, ok
}

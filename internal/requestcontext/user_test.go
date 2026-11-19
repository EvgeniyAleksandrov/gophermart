package requestcontext_test

import (
	"context"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/stretchr/testify/require"
)

func TestUserWithIDAndID(t *testing.T) {
	user := requestcontext.NewUser()
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
	defer cancel()

	id, ok := user.ID(parent)
	require.False(t, ok)
	require.Zero(t, id)

	derived := user.WithID(parent, 42)
	id, ok = user.ID(derived)
	require.True(t, ok)
	require.Equal(t, int64(42), id)
	_, ok = user.ID(parent)
	require.False(t, ok)

	replaced := user.WithID(derived, 43)
	id, ok = user.ID(replaced)
	require.True(t, ok)
	require.Equal(t, int64(43), id)
	id, _ = user.ID(derived)
	require.Equal(t, int64(42), id)

	deadline, _ := parent.Deadline()
	derivedDeadline, _ := derived.Deadline()
	require.Equal(t, deadline, derivedDeadline)
	cancel()
	require.ErrorIs(t, derived.Err(), context.Canceled)
}

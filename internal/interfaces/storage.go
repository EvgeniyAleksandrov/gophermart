package interfaces

import (
	"context"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=storage.go -destination=mock/storage_mock.go -package=mock

// DBStorage exposes database operations to consumers, excluding lifecycle ownership.
type DBStorage interface {
	// AddUser atomically creates a user and a zero balance, returning only after commit.
	AddUser(ctx context.Context, login, passwordHash string) (model.User, error)
	// FindUserByLogin returns credentials for the exact, case-sensitive login.
	FindUserByLogin(ctx context.Context, login string) (model.User, error)
}

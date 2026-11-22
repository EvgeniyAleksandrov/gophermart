package interfaces

import (
	"context"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=storage.go -destination=mock/storage_mock.go -package=mock

// DBStorage exposes database operations to consumers, excluding lifecycle ownership.
type DBStorage interface {
	// UserExists checks whether the user is present at the time of the query.
	UserExists(ctx context.Context, userID int64) (bool, error)
	// AddOrder returns true for a new order, false for a repeat by its owner.
	AddOrder(ctx context.Context, userID int64, number string) (bool, error)
	// ListOrdersByUser returns orders sorted by upload time and internal ID descending.
	ListOrdersByUser(ctx context.Context, userID int64) ([]model.Order, error)
	// AddUser atomically creates a user and a zero balance, returning only after commit.
	AddUser(ctx context.Context, login, passwordHash string) (model.User, error)
	// FindUserByLogin returns credentials for the exact, case-sensitive login.
	FindUserByLogin(ctx context.Context, login string) (model.User, error)
}

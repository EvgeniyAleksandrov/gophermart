//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=storage.go -destination=mock/storage_mock.go -package=mock

package interfaces

import (
	"context"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type DBStorage interface {
	// Withdraw atomically debits the balance; matching repeats leave all data unchanged.
	Withdraw(ctx context.Context, userID int64, number string, sum int64) error
	// ListWithdrawalsByUser returns complete history by processed time and ID descending.
	ListWithdrawalsByUser(ctx context.Context, userID int64) ([]model.Withdrawal, error)

	// ListPending returns all unfinished orders in ID order.
	ListPending(context.Context) ([]model.Order, error)
	// ApplyAccrual updates order and balance atomically and idempotently.
	ApplyAccrual(context.Context, int64, int64, model.AccrualResult) error
	// GetBalance reads the user's balance in hundredths.
	GetBalance(context.Context, int64) (model.Balance, error)

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

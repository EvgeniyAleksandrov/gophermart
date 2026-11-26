//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=interfaces.go -destination=interfaces_mock_test.go -package=handler_test

// Package handler implements the HTTP API.
package handler

import (
	"context"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

type OrderService interface {
	Upload(ctx context.Context, userID int64, rawNumber string) (bool, error)
	List(ctx context.Context, userID int64) ([]model.Order, error)
}

type WithdrawalService interface {
	Withdraw(ctx context.Context, userID int64, number string, sum int64) error
	List(ctx context.Context, userID int64) ([]model.Withdrawal, error)
}

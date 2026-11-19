// Package handler implements the HTTP API.
package handler

import "context"

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=interfaces.go -destination=interfaces_mock_test.go -package=handler_test

// AuthService provides registration and login without exposing storage details.
type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

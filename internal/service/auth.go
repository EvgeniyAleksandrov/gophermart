//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=auth.go -destination=auth_mock_test.go -package=service_test

// Package service implements business operations over storage and authentication contracts.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type PasswordHasher interface {
	Hash(password string) string
	Matches(password, hash string) bool
}

type TokenIssuer interface {
	Issue(userID int64) (string, error)
}

type AuthService struct {
	storage interfaces.DBStorage
	hasher  PasswordHasher
	tokens  TokenIssuer
}

func NewAuthService(storage interfaces.DBStorage, hasher PasswordHasher, tokens TokenIssuer) *AuthService {
	return &AuthService{
		storage: storage,
		hasher:  hasher,
		tokens:  tokens,
	}
}

func (s *AuthService) Register(ctx context.Context, login, password string) (string, error) {
	if login == "" || password == "" {
		return "", model.ErrInvalidCredentialsInput
	}

	user, err := s.storage.AddUser(ctx, login, s.hasher.Hash(password))
	if err != nil {
		return "", fmt.Errorf("register user: %w", err)
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return "", fmt.Errorf("issue registration token: %w", err)
	}

	return token, nil
}

func (s *AuthService) Login(ctx context.Context, login, password string) (string, error) {
	if login == "" || password == "" {
		return "", model.ErrInvalidCredentialsInput
	}

	user, err := s.storage.FindUserByLogin(ctx, login)
	if errors.Is(err, model.ErrUserNotFound) {
		return "", model.ErrInvalidCredentials
	}

	if err != nil {
		return "", fmt.Errorf("find login user: %w", err)
	}

	if !s.hasher.Matches(password, user.PasswordHash) {
		return "", model.ErrInvalidCredentials
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		return "", fmt.Errorf("issue login token: %w", err)
	}

	return token, nil
}

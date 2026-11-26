package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Balance struct {
	storage interfaces.DBStorage
}

func NewBalance(storage interfaces.DBStorage) *Balance { return &Balance{storage: storage} }

func (s *Balance) Get(ctx context.Context, userID int64) (model.Balance, error) {
	if err := s.checkUser(ctx, userID); err != nil {
		return model.Balance{}, err
	}

	balance, err := s.storage.GetBalance(ctx, userID)
	if errors.Is(err, model.ErrBalanceNotFound) {
		if userErr := s.checkUser(ctx, userID); userErr != nil {
			return model.Balance{}, userErr
		}
	}

	if err != nil {
		return model.Balance{}, fmt.Errorf("get user balance: %w", err)
	}

	return balance, nil
}

func (s *Balance) checkUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return model.ErrUserNotFound
	}

	exists, err := s.storage.UserExists(ctx, userID)
	if err != nil {
		return fmt.Errorf("check balance user: %w", err)
	}

	if !exists {
		return model.ErrUserNotFound
	}

	return nil
}

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Order struct {
	storage   interfaces.DBStorage
	validator OrderNumberChecker
}

func NewOrder(storage interfaces.DBStorage, validator OrderNumberChecker) *Order {
	return &Order{
		storage:   storage,
		validator: validator,
	}
}

func (s *Order) Upload(ctx context.Context, userID int64, rawNumber string) (bool, error) {
	if userID <= 0 {
		return false, model.ErrUserNotFound
	}

	number := strings.TrimSpace(rawNumber)
	if err := s.validator.Validate(number); err != nil {
		return false, fmt.Errorf("validate order number: %w", err)
	}

	if err := s.checkUser(ctx, userID); err != nil {
		return false, err
	}

	created, err := s.storage.AddOrder(ctx, userID, number)
	if err != nil {
		return false, fmt.Errorf("upload order: %w", err)
	}

	return created, nil
}

func (s *Order) List(ctx context.Context, userID int64) ([]model.Order, error) {
	if err := s.checkUser(ctx, userID); err != nil {
		return nil, err
	}

	orders, err := s.storage.ListOrdersByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user orders: %w", err)
	}

	return orders, nil
}

func (s *Order) checkUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return model.ErrUserNotFound
	}

	exists, err := s.storage.UserExists(ctx, userID)
	if err != nil {
		return fmt.Errorf("check order user: %w", err)
	}

	if !exists {
		return model.ErrUserNotFound
	}

	return nil
}

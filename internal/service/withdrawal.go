package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Withdrawal struct {
	storage   interfaces.DBStorage
	validator OrderNumberChecker
}

func NewWithdrawal(storage interfaces.DBStorage, validator OrderNumberChecker) *Withdrawal {
	return &Withdrawal{storage: storage, validator: validator}
}

func (s *Withdrawal) Withdraw(ctx context.Context, userID int64, number string, sum int64) error {
	if sum <= 0 {
		return model.ErrInvalidWithdrawalAmount
	}

	if err := s.validator.Validate(number); err != nil {
		if errors.Is(err, model.ErrEmptyOrderNumber) {
			return model.ErrInvalidOrderNumber
		}

		return fmt.Errorf("validate withdrawal number: %w", err)
	}

	if err := s.checkUser(ctx, userID); err != nil {
		return err
	}

	if err := s.storage.Withdraw(ctx, userID, number, sum); err != nil {
		return fmt.Errorf("withdraw points: %w", err)
	}

	return nil
}

func (s *Withdrawal) List(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	if err := s.checkUser(ctx, userID); err != nil {
		return nil, err
	}

	withdrawals, err := s.storage.ListWithdrawalsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user withdrawals: %w", err)
	}

	return withdrawals, nil
}

func (s *Withdrawal) checkUser(ctx context.Context, userID int64) error {
	if userID <= 0 {
		return model.ErrUserNotFound
	}

	exists, err := s.storage.UserExists(ctx, userID)
	if err != nil {
		return fmt.Errorf("check withdrawal user: %w", err)
	}

	if !exists {
		return model.ErrUserNotFound
	}

	return nil
}

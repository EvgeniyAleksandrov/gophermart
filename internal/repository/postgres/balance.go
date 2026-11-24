package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

func (s *DBStorage) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	var result model.Balance
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.getBalance(ctx, userID)
		return err
	})
	if err != nil {
		return model.Balance{}, err
	}

	return result, nil
}

func (s *DBStorage) getBalance(ctx context.Context, userID int64) (model.Balance, error) {
	var balance model.Balance
	err := s.db.QueryRowContext(
		ctx,
		`SELECT current, withdrawn FROM public.gophermart_balances WHERE user_id = $1`,
		userID,
	).Scan(&balance.Current, &balance.Withdrawn)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Balance{}, model.ErrBalanceNotFound
	}

	if err != nil {
		return model.Balance{}, fmt.Errorf("read balance: %w", err)
	}

	return balance, nil
}

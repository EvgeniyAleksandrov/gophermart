package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

// GetBalance returns a user's balance or ErrBalanceNotFound, without partial data.
func (s *DBStorage) GetBalance(ctx context.Context, userID int64) (model.Balance, error) {
	op := &getBalance{userID: userID}
	if err := s.execute(ctx, op); err != nil {
		return model.Balance{}, err
	}

	return op.result, nil
}

type getBalance struct {
	userID int64
	result model.Balance
}

func (o *getBalance) CanReplayUnknownOutcome() bool { return true }

func (o *getBalance) Execute(ctx context.Context, db *sql.DB) error {
	o.result = model.Balance{}
	var balance model.Balance
	err := db.QueryRowContext(ctx, `SELECT current, withdrawn FROM public.gophermart_balances WHERE user_id = $1`, o.userID).Scan(&balance.Current, &balance.Withdrawn)
	if errors.Is(err, sql.ErrNoRows) {
		return model.ErrBalanceNotFound
	}

	if err != nil {
		return fmt.Errorf("read balance: %w", err)
	}
	o.result = balance
	return nil
}

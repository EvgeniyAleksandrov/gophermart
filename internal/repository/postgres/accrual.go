package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

// ApplyAccrual atomically updates an order and its balance; final orders are unchanged.
func (s *DBStorage) ApplyAccrual(ctx context.Context, orderID, userID int64, result model.AccrualResult) error {
	return s.execute(ctx, &applyAccrual{orderID: orderID, userID: userID, result: result})
}

type applyAccrual struct {
	orderID, userID int64
	result          model.AccrualResult
}

func (o *applyAccrual) CanReplayUnknownOutcome() bool { return true }

func (o *applyAccrual) Execute(ctx context.Context, db *sql.DB) (err error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin accrual transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback accrual: %w", rollbackErr))
		}
	}()
	var current int64
	err = tx.QueryRowContext(ctx, `SELECT current FROM public.gophermart_balances WHERE user_id = $1 FOR UPDATE`, o.userID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM public.gophermart_users WHERE id = $1)`, o.userID).Scan(&exists); err != nil {
			return fmt.Errorf("check accrual user: %w", err)
		}

		if !exists {
			return nil
		}

		return model.ErrBalanceNotFound
	}

	if err != nil {
		return fmt.Errorf("lock balance: %w", err)
	}

	var ownerID int64
	var number string
	var status model.OrderStatus
	err = tx.QueryRowContext(ctx, `SELECT user_id, number, status FROM public.gophermart_orders WHERE id = $1 FOR UPDATE`, o.orderID).Scan(&ownerID, &number, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("lock order: %w", err)
	}

	if ownerID != o.userID || number != o.result.Number {
		return errors.New("apply accrual: order identity mismatch")
	}

	if status == model.OrderStatusInvalid || status == model.OrderStatusProcessed {
		return nil
	}

	var amount any
	next := model.OrderStatusProcessing
	switch o.result.Status {
	case model.AccrualRegistered, model.AccrualProcessing:
	case model.AccrualInvalid:
		next, amount = model.OrderStatusInvalid, int64(0)
	case model.AccrualProcessed:
		if o.result.Accrual < 0 || current < 0 || o.result.Accrual > math.MaxInt64-current {
			return errors.New("apply accrual: balance overflow")
		}
		next, amount = model.OrderStatusProcessed, o.result.Accrual
		if _, err := tx.ExecContext(ctx, `UPDATE public.gophermart_balances SET current = $1 WHERE user_id = $2`, current+o.result.Accrual, o.userID); err != nil {
			return fmt.Errorf("credit balance: %w", err)
		}
	default:
		return model.ErrAccrualInvalidResponse
	}

	if _, err := tx.ExecContext(ctx, `UPDATE public.gophermart_orders SET status = $1, accrual = $2 WHERE id = $3`, next, amount, o.orderID); err != nil {
		return fmt.Errorf("update accrued order: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit accrual: %w", (&writeOutcome{}).classify(err))
	}

	return nil
}

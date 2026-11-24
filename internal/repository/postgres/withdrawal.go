package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

func (s *DBStorage) Withdraw(ctx context.Context, userID int64, number string, sum int64) error {
	if sum <= 0 {
		return model.ErrInvalidWithdrawalAmount
	}

	return s.retryer.Do(ctx, func() error {
		return s.withdraw(ctx, userID, number, sum)
	})
}

func (s *DBStorage) withdraw(ctx context.Context, userID int64, number string, sum int64) (err error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin withdrawal transaction: %w", err)
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback withdrawal: %w", rollbackErr))
		}
	}()

	var current, withdrawn int64
	err = tx.QueryRowContext(
		ctx,
		`SELECT current, withdrawn FROM public.gophermart_balances WHERE user_id = $1 FOR UPDATE`,
		userID,
	).Scan(&current, &withdrawn)
	if errors.Is(err, sql.ErrNoRows) {
		var exists bool
		if err := tx.QueryRowContext(
			ctx,
			`SELECT EXISTS (SELECT 1 FROM public.gophermart_users WHERE id = $1)`,
			userID,
		).Scan(&exists); err != nil {
			return fmt.Errorf("check withdrawal user: %w", err)
		}

		if !exists {
			return model.ErrUserNotFound
		}

		return model.ErrBalanceNotFound
	}

	if err != nil {
		return fmt.Errorf("lock withdrawal balance: %w", err)
	}

	err = s.checkExistingWithdrawal(ctx, tx, userID, number, sum)
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO public.gophermart_withdrawals (user_id, order_number, sum)
 VALUES ($1, $2, $3) ON CONFLICT (order_number) DO NOTHING RETURNING id`, userID, number, sum).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		// A fresh statement snapshot sees the winner of the competing insertion.
		return s.checkExistingWithdrawal(ctx, tx, userID, number, sum)
	}

	if err != nil {
		return fmt.Errorf("insert withdrawal: %w", err)
	}

	if current < sum {
		return model.ErrInsufficientFunds
	}

	if withdrawn > math.MaxInt64-sum {
		return errors.New("withdraw points: withdrawn overflow")
	}

	result, err := tx.ExecContext(ctx, `UPDATE public.gophermart_balances
 SET current = current - $2, withdrawn = withdrawn + $2 WHERE user_id = $1`, userID, sum)
	if err != nil {
		return fmt.Errorf("debit balance: %w", err)
	}

	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count debited balances: %w", err)
	}

	if changed != 1 {
		return fmt.Errorf("debit balance: expected one row, got %d", changed)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit withdrawal: %w", s.classifyWriteOutcome(err))
	}

	return nil
}

func (s *DBStorage) checkExistingWithdrawal(
	ctx context.Context,
	tx *sql.Tx,
	userID int64,
	number string,
	sum int64,
) error {
	var ownerID, existingSum int64
	err := tx.QueryRowContext(
		ctx,
		`SELECT user_id, sum FROM public.gophermart_withdrawals WHERE order_number = $1`,
		number,
	).Scan(&ownerID, &existingSum)
	if err != nil {
		return fmt.Errorf("read existing withdrawal: %w", err)
	}

	if ownerID != userID || existingSum != sum {
		return model.ErrWithdrawalConflict
	}

	return nil
}

func (s *DBStorage) ListWithdrawalsByUser(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	var result []model.Withdrawal
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.listWithdrawalsByUser(ctx, userID)
		return err
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *DBStorage) listWithdrawalsByUser(
	ctx context.Context,
	userID int64,
) (result []model.Withdrawal, err error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, order_number, sum, processed_at
 FROM public.gophermart_withdrawals WHERE user_id = $1 ORDER BY processed_at DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}

	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close withdrawals: %w", closeErr))
		}

		if err != nil {
			result = nil
		}
	}()

	var withdrawals []model.Withdrawal

	for rows.Next() {
		var withdrawal model.Withdrawal
		if err := rows.Scan(
			&withdrawal.ID,
			&withdrawal.UserID,
			&withdrawal.OrderNumber,
			&withdrawal.Sum,
			&withdrawal.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("scan withdrawal: %w", err)
		}

		withdrawals = append(withdrawals, withdrawal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read withdrawals: %w", err)
	}

	return withdrawals, nil
}

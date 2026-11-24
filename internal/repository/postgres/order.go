package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *DBStorage) AddOrder(ctx context.Context, userID int64, number string) (bool, error) {
	var result bool
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.addOrder(ctx, userID, number)

		var unknown *OutcomeUnknownError
		if errors.As(err, &unknown) {
			return &retry.PermanentError{Err: err}
		}

		return err
	})
	if err != nil {
		return false, err
	}

	return result, nil
}

func (s *DBStorage) addOrder(ctx context.Context, userID int64, number string) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
 INSERT INTO public.gophermart_orders (user_id, number)
 VALUES ($1, $2) ON CONFLICT (number) DO NOTHING RETURNING id`, userID, number).Scan(&id)
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		var pgErr *pgconn.PgError
		if errors.As(
			err,
			&pgErr,
		) && pgErr.Code == "23503" && pgErr.ConstraintName == "gophermart_orders_user_fk" {
			return false, fmt.Errorf("add order: %w", model.ErrUserNotFound)
		}

		return false, fmt.Errorf("add order: %w", s.classifyWriteOutcome(err))
	}

	var ownerID int64
	err = s.db.QueryRowContext(ctx, `
 SELECT user_id FROM public.gophermart_orders WHERE number = $1`, number).Scan(&ownerID)
	if err != nil {
		return false, fmt.Errorf("find existing order owner: %w", err)
	}

	if ownerID != userID {
		return false, model.ErrOrderOwnedByAnotherUser
	}

	return false, nil
}

func (s *DBStorage) ListOrdersByUser(ctx context.Context, userID int64) ([]model.Order, error) {
	var result []model.Order
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.listOrders(ctx, `SELECT id, user_id, number, status, accrual, uploaded_at
 FROM public.gophermart_orders WHERE user_id = $1 ORDER BY uploaded_at DESC, id DESC`, userID)
		return err
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *DBStorage) listOrders(ctx context.Context, query string, args ...any) ([]model.Order, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}

	defer rows.Close()

	var orders []model.Order

	for rows.Next() {
		var order model.Order
		var accrual sql.NullInt64
		if err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.Number,
			&order.Status,
			&accrual,
			&order.UploadedAt,
		); err != nil {
			return nil, errors.Join(fmt.Errorf("scan order: %w", err), rows.Close())
		}

		if accrual.Valid {
			order.Accrual = &accrual.Int64
		}

		orders = append(orders, order)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read orders: %w", err)
	}

	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close orders: %w", err)
	}

	return orders, nil
}

func (s *DBStorage) ListPending(ctx context.Context) ([]model.Order, error) {
	var result []model.Order
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.listOrders(ctx, `SELECT id, user_id, number, status, accrual, uploaded_at
 FROM public.gophermart_orders WHERE status IN ('NEW', 'PROCESSING') ORDER BY id ASC`)
		return err
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

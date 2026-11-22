package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
)

// AddOrder inserts a NEW order with database defaults, without changing repeats.
// A separate SELECT sees a committed competing insert after ON CONFLICT waits.
// Only the order's user foreign key maps to ErrUserNotFound. No retries are made:
// a failed write may have an unknown outcome and cannot promise created vs repeated.
func (s *DBStorage) AddOrder(ctx context.Context, userID int64, number string) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
 INSERT INTO public.gophermart_orders (user_id, number)
 VALUES ($1, $2) ON CONFLICT (number) DO NOTHING RETURNING id`, userID, number).Scan(&id)
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "gophermart_orders_user_fk" {
			return false, fmt.Errorf("add order: %w", model.ErrUserNotFound)
		}

		return false, fmt.Errorf("add order: %w", err)
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

// ListOrdersByUser reads only this user's orders, newest first with ID as tie-breaker.
// It returns no partial results on query, scan or iteration failures.
func (s *DBStorage) ListOrdersByUser(ctx context.Context, userID int64) ([]model.Order, error) {
	rows, err := s.db.QueryContext(ctx, `
 SELECT id, user_id, number, status, accrual, uploaded_at
 FROM public.gophermart_orders WHERE user_id = $1
 ORDER BY uploaded_at DESC, id DESC`, userID)
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

	return orders, nil
}

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
// Only the order's user foreign key maps to ErrUserNotFound. Unknown write outcomes
// are not replayed, preserving the distinction between creation and repetition.
func (s *DBStorage) AddOrder(ctx context.Context, userID int64, number string) (bool, error) {
	op := &addOrder{userID: userID, number: number}
	if err := s.execute(ctx, op); err != nil {
		return false, err
	}

	return op.result, nil
}

type addOrder struct {
	userID int64
	number string
	result bool
}

func (o *addOrder) CanReplayUnknownOutcome() bool { return false }

func (o *addOrder) Execute(ctx context.Context, db *sql.DB) error {
	o.result = false
	result, err := o.run(ctx, db)
	if err == nil {
		o.result = result
	}

	return err
}

func (o *addOrder) run(ctx context.Context, db *sql.DB) (bool, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
 INSERT INTO public.gophermart_orders (user_id, number)
 VALUES ($1, $2) ON CONFLICT (number) DO NOTHING RETURNING id`, o.userID, o.number).Scan(&id)
	if err == nil {
		return true, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "gophermart_orders_user_fk" {
			return false, fmt.Errorf("add order: %w", model.ErrUserNotFound)
		}

		return false, fmt.Errorf("add order: %w", (&writeOutcome{}).classify(err))
	}

	var ownerID int64
	err = db.QueryRowContext(ctx, `
 SELECT user_id FROM public.gophermart_orders WHERE number = $1`, o.number).Scan(&ownerID)
	if err != nil {
		return false, fmt.Errorf("find existing order owner: %w", err)
	}

	if ownerID != o.userID {
		return false, model.ErrOrderOwnedByAnotherUser
	}

	return false, nil
}

// ListOrdersByUser reads only this user's orders, newest first with ID as tie-breaker.
// It returns no partial results on query, scan or iteration failures.
func (s *DBStorage) ListOrdersByUser(ctx context.Context, userID int64) ([]model.Order, error) {
	op := &listOrders{userID: userID}
	if err := s.execute(ctx, op); err != nil {
		return nil, err
	}

	return op.result, nil
}

type listOrders struct {
	pending bool
	userID  int64
	result  []model.Order
}

func (o *listOrders) CanReplayUnknownOutcome() bool { return true }

func (o *listOrders) Execute(ctx context.Context, db *sql.DB) error {
	o.result = nil
	result, err := o.run(ctx, db)
	if err == nil {
		o.result = result
	}

	return err
}

func (o *listOrders) run(ctx context.Context, db *sql.DB) ([]model.Order, error) {
	query := `SELECT id, user_id, number, status, accrual, uploaded_at
 FROM public.gophermart_orders WHERE user_id = $1 ORDER BY uploaded_at DESC, id DESC`
	args := []any{o.userID}
	if o.pending {
		query = `SELECT id, user_id, number, status, accrual, uploaded_at
 FROM public.gophermart_orders WHERE status IN ('NEW', 'PROCESSING') ORDER BY id ASC`
		args = nil
	}
	rows, err := db.QueryContext(ctx, query, args...)
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

// ListPending fully reads all unfinished orders in ID order before returning.
func (s *DBStorage) ListPending(ctx context.Context) ([]model.Order, error) {
	op := &listOrders{pending: true}
	if err := s.execute(ctx, op); err != nil {
		return nil, err
	}

	return op.result, nil
}

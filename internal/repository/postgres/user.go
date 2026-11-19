package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/jackc/pgx/v5/pgconn"
)

// AddUser atomically creates a user and its zero balance at Read Committed isolation.
// Only the login constraint maps to ErrLoginTaken. Commit errors are not retried:
// a failed commit may have an unknown outcome. Cleanup errors remain discoverable.
func (s *DBStorage) AddUser(ctx context.Context, login, passwordHash string) (_ model.User, err error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return model.User{}, fmt.Errorf("begin user transaction: %w", err)
	}

	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("rollback user transaction: %w", rollbackErr))
		}
	}()

	var user model.User
	err = tx.QueryRowContext(ctx, `
  INSERT INTO public.gophermart_users (login, password_hash)
  VALUES ($1, $2) RETURNING id, login, password_hash`, login, passwordHash).
		Scan(&user.ID, &user.Login, &user.PasswordHash)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "gophermart_users_login_key" {
			return model.User{}, fmt.Errorf("create user: %w", model.ErrLoginTaken)
		}

		return model.User{}, fmt.Errorf("create user: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
  INSERT INTO public.gophermart_balances (user_id, current, withdrawn)
  VALUES ($1, 0, 0)`, user.ID); err != nil {
		return model.User{}, fmt.Errorf("create user balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.User{}, fmt.Errorf("commit user transaction: %w", err)
	}

	return user, nil
}

// FindUserByLogin reads credentials without normalizing the login or comparing passwords.
// A missing row maps to ErrUserNotFound; SQL errors remain internal failures.
func (s *DBStorage) FindUserByLogin(ctx context.Context, login string) (model.User, error) {
	var user model.User
	err := s.db.QueryRowContext(ctx, `
  SELECT id, login, password_hash FROM public.gophermart_users WHERE login = $1`, login).
		Scan(&user.ID, &user.Login, &user.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, model.ErrUserNotFound
	}

	if err != nil {
		return model.User{}, fmt.Errorf("find user by login: %w", err)
	}

	return user, nil
}

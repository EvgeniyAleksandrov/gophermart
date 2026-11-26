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

func (s *DBStorage) AddUser(ctx context.Context, login, passwordHash string) (model.User, error) {
	var result model.User
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.addUser(ctx, login, passwordHash)

		var unknown *OutcomeUnknownError
		if errors.As(err, &unknown) {
			return &retry.PermanentError{Err: err}
		}

		return err
	})
	if err != nil {
		return model.User{}, err
	}

	return result, nil
}

func (s *DBStorage) addUser(ctx context.Context, login, passwordHash string) (_ model.User, err error) {
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
		if errors.As(
			err,
			&pgErr,
		) && pgErr.Code == "23505" && pgErr.ConstraintName == "gophermart_users_login_key" {
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
		return model.User{}, fmt.Errorf("commit user transaction: %w", s.classifyWriteOutcome(err))
	}

	return user, nil
}

func (s *DBStorage) FindUserByLogin(ctx context.Context, login string) (model.User, error) {
	var result model.User
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.findUserByLogin(ctx, login)
		return err
	})
	if err != nil {
		return model.User{}, err
	}

	return result, nil
}

func (s *DBStorage) findUserByLogin(ctx context.Context, login string) (model.User, error) {
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

func (s *DBStorage) UserExists(ctx context.Context, userID int64) (bool, error) {
	var result bool
	err := s.retryer.Do(ctx, func() error {
		var err error
		result, err = s.userExists(ctx, userID)
		return err
	})
	if err != nil {
		return false, err
	}

	return result, nil
}

func (s *DBStorage) userExists(ctx context.Context, userID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
 SELECT EXISTS (SELECT 1 FROM public.gophermart_users WHERE id = $1)`, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user existence: %w", err)
	}

	return exists, nil
}

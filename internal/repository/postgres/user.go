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
// Only the login constraint maps to ErrLoginTaken. Unknown commit outcomes are not
// replayed. Each retry starts a new transaction; cleanup errors remain discoverable.
func (s *DBStorage) AddUser(ctx context.Context, login, passwordHash string) (model.User, error) {
	op := &addUser{login: login, passwordHash: passwordHash}
	if err := s.execute(ctx, op); err != nil {
		return model.User{}, err
	}

	return op.result, nil
}

type addUser struct {
	login, passwordHash string
	result              model.User
}

func (o *addUser) CanReplayUnknownOutcome() bool { return false }

func (o *addUser) Execute(ctx context.Context, db *sql.DB) error {
	o.result = model.User{}
	result, err := o.run(ctx, db)
	if err == nil {
		o.result = result
	}

	return err
}

func (o *addUser) run(ctx context.Context, db *sql.DB) (_ model.User, err error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
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
  VALUES ($1, $2) RETURNING id, login, password_hash`, o.login, o.passwordHash).
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
		return model.User{}, fmt.Errorf("commit user transaction: %w", (&writeOutcome{}).classify(err))
	}

	return user, nil
}

// FindUserByLogin reads credentials without normalizing the login or comparing passwords.
// A missing row maps to ErrUserNotFound; SQL errors remain internal failures.
func (s *DBStorage) FindUserByLogin(ctx context.Context, login string) (model.User, error) {
	op := &findUser{login: login}
	if err := s.execute(ctx, op); err != nil {
		return model.User{}, err
	}

	return op.result, nil
}

type findUser struct {
	login  string
	result model.User
}

func (o *findUser) CanReplayUnknownOutcome() bool { return true }

func (o *findUser) Execute(ctx context.Context, db *sql.DB) error {
	o.result = model.User{}
	result, err := o.run(ctx, db)
	if err == nil {
		o.result = result
	}

	return err
}

func (o *findUser) run(ctx context.Context, db *sql.DB) (model.User, error) {
	var user model.User
	err := db.QueryRowContext(ctx, `
  SELECT id, login, password_hash FROM public.gophermart_users WHERE login = $1`, o.login).
		Scan(&user.ID, &user.Login, &user.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return model.User{}, model.ErrUserNotFound
	}

	if err != nil {
		return model.User{}, fmt.Errorf("find user by login: %w", err)
	}

	return user, nil
}

// UserExists checks only the user's primary key, without reading credentials.
func (s *DBStorage) UserExists(ctx context.Context, userID int64) (bool, error) {
	op := &userExists{userID: userID}
	if err := s.execute(ctx, op); err != nil {
		return false, err
	}

	return op.result, nil
}

type userExists struct {
	userID int64
	result bool
}

func (o *userExists) CanReplayUnknownOutcome() bool { return true }

func (o *userExists) Execute(ctx context.Context, db *sql.DB) error {
	o.result = false
	result, err := o.run(ctx, db)
	if err == nil {
		o.result = result
	}

	return err
}

func (o *userExists) run(ctx context.Context, db *sql.DB) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
 SELECT EXISTS (SELECT 1 FROM public.gophermart_users WHERE id = $1)`, o.userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check user existence: %w", err)
	}

	return exists, nil
}

package model

import "errors"

var (
	// ErrInvalidCredentialsInput indicates an empty login or password.
	ErrInvalidCredentialsInput = errors.New("login and password must be nonempty")
	// ErrLoginTaken indicates a conflict with the unique login constraint.
	ErrLoginTaken = errors.New("login already taken")
	// ErrUserNotFound indicates that a user lookup returned no row.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidCredentials hides whether the login or password failed authentication.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

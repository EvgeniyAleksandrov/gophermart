package model

import "errors"

var (
	// ErrAccrualNotRegistered means the external service does not know the order yet.
	ErrAccrualNotRegistered = errors.New("accrual order not registered")
	// ErrAccrualUnavailable means an external request failed temporarily.
	ErrAccrualUnavailable = errors.New("accrual unavailable")
	// ErrAccrualInvalidResponse means the external response cannot be applied.
	ErrAccrualInvalidResponse = errors.New("invalid accrual response")
	// ErrBalanceNotFound indicates a missing mandatory balance row.
	ErrBalanceNotFound = errors.New("balance not found")

	// ErrEmptyOrderNumber indicates an empty order number after trimming.
	ErrEmptyOrderNumber = errors.New("empty order number")
	// ErrInvalidOrderNumber indicates invalid digits, leading zeros or checksum.
	ErrInvalidOrderNumber = errors.New("invalid order number")
	// ErrOrderOwnedByAnotherUser indicates a number already claimed by another user.
	ErrOrderOwnedByAnotherUser = errors.New("order already belongs to another user")
	// ErrInvalidCredentialsInput indicates an empty login or password.
	ErrInvalidCredentialsInput = errors.New("login and password must be nonempty")
	// ErrLoginTaken indicates a conflict with the unique login constraint.
	ErrLoginTaken = errors.New("login already taken")
	// ErrUserNotFound indicates that a user lookup returned no row.
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidCredentials hides whether the login or password failed authentication.
	ErrInvalidCredentials = errors.New("invalid credentials")
)

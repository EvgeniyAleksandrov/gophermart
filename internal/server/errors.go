package server

import "errors"

var (
	// ErrMethodUnsupported indicates a method outside chi's built-in method set.
	ErrMethodUnsupported = errors.New("unsupported HTTP method")

	// ErrPathEmpty indicates that a route has no path.
	ErrPathEmpty = errors.New("path is empty")

	// ErrPathFormat indicates a path without a leading slash.
	ErrPathFormat = errors.New("path must start with /")

	// ErrHandlerNil indicates that no handler was supplied.
	ErrHandlerNil = errors.New("handler is nil")
)

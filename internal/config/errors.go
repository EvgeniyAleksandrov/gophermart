package config

import "errors"

var (
	// ErrConfigNil indicates a missing configuration.
	ErrConfigNil = errors.New("configuration is nil")

	// ErrRunAddressEmpty indicates an empty listen address.
	ErrRunAddressEmpty = errors.New("run address is empty")

	// ErrRunAddressWhitespace indicates whitespace in the listen address.
	ErrRunAddressWhitespace = errors.New("whitespace is not allowed")

	// ErrRunAddressFormat indicates a listen address without valid host:port syntax.
	ErrRunAddressFormat = errors.New("expected host:port")

	// ErrRunAddressInvalidIP indicates an invalid bracketed IP in the listen address.
	ErrRunAddressInvalidIP = errors.New("invalid bracketed IP address")

	// ErrPortEmpty indicates a missing port value.
	ErrPortEmpty = errors.New("port is empty")

	// ErrPortNotNumeric indicates a port containing nondecimal characters.
	ErrPortNotNumeric = errors.New("port must contain only decimal digits")

	// ErrPortOutOfRange indicates a port outside the range 1–65535.
	ErrPortOutOfRange = errors.New("port must be between 1 and 65535")

	// ErrDatabaseURIEmpty indicates an empty PostgreSQL connection string.
	ErrDatabaseURIEmpty = errors.New("database connection string is empty")

	// ErrDatabaseSchemeUnsupported indicates an unsupported database URL scheme.
	ErrDatabaseSchemeUnsupported = errors.New("unsupported database URL scheme")

	// ErrDatabaseURIInvalid indicates a connection string rejected by pgx.
	ErrDatabaseURIInvalid = errors.New("invalid database connection string")

	// ErrAccrualAddressEmpty indicates an empty accrual address.
	ErrAccrualAddressEmpty = errors.New("accrual address is empty")

	// ErrAccrualAddressInvalid indicates malformed or opaque accrual URL syntax.
	ErrAccrualAddressInvalid = errors.New("invalid accrual URL")

	// ErrAccrualScheme indicates a scheme other than HTTP or HTTPS.
	ErrAccrualScheme = errors.New("expected HTTP or HTTPS scheme")

	// ErrAccrualHostEmpty indicates a missing accrual hostname.
	ErrAccrualHostEmpty = errors.New("hostname is empty")

	// ErrAccrualCredentials indicates credentials in the accrual URL.
	ErrAccrualCredentials = errors.New("credentials are not allowed")

	// ErrAccrualPath indicates an accrual URL path other than empty or /.
	ErrAccrualPath = errors.New("path must be empty or /")

	// ErrAccrualQuery indicates a query component in the accrual URL.
	ErrAccrualQuery = errors.New("query is not allowed")

	// ErrAccrualFragment indicates a fragment component in the accrual URL.
	ErrAccrualFragment = errors.New("fragment is not allowed")

	// ErrAccrualWhitespace indicates whitespace in the accrual URL.
	ErrAccrualWhitespace = errors.New("whitespace is not allowed")

	// ErrAccrualInvalidIP indicates an invalid IP literal in the accrual URL.
	ErrAccrualInvalidIP = errors.New("invalid bracketed IP address")
)

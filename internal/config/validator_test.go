package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testDSN = "host=localhost port=5432 user=postgres password=test-secret dbname=gophermart sslmode=disable"

func TestValidator(t *testing.T) {
	testCases := map[string]struct {
		field string
		value string
		valid bool
	}{
		"empty host": {
			field: "run",
			value: ":8080",
			valid: true,
		},
		"hostname": {
			field: "run",
			value: "localhost:8080",
			valid: true,
		},
		"ipv6": {
			field: "run",
			value: "[::1]:8080",
			valid: true,
		},
		"missing port": {
			field: "run",
			value: "localhost",
			valid: false,
		},
		"zero": {
			field: "run",
			value: ":0",
			valid: false,
		},
		"large": {
			field: "run",
			value: ":65536",
			valid: false,
		},
		"text port": {
			field: "run",
			value: ":http",
			valid: false,
		},
		"signed port": {
			field: "run",
			value: ":+80",
			valid: false,
		},
		"space": {
			field: "run",
			value: "local host:80",
			valid: false,
		},
		"url": {
			field: "run",
			value: "http://localhost:80",
			valid: false,
		},
		"bad brackets": {
			field: "run",
			value: "[localhost]:80",
			valid: false,
		},
		"keyword": {
			field: "db",
			value: testDSN,
			valid: true,
		},
		"keyword spaces": {
			field: "db",
			value: "host = localhost port = 5432 user = postgres dbname = gophermart sslmode = disable",
			valid: true,
		},
		"postgres": {
			field: "db",
			value: "postgres://postgres:test-secret@localhost:5432/gophermart?sslmode=disable",
			valid: true,
		},
		"postgresql": {
			field: "db",
			value: "postgresql://postgres:test-secret@localhost:5432/gophermart?sslmode=disable",
			valid: true,
		},
		"empty db": {
			field: "db",
			value: "",
			valid: false,
		},
		"blank db": {
			field: "db",
			value: "  ",
			valid: false,
		},
		"wrong scheme": {
			field: "db",
			value: "https://localhost/db?password=test-secret",
			valid: false,
		},
		"invalid dsn": {
			field: "db",
			value: "host=localhost password='test-secret",
			valid: false,
		},
		"sslmode": {
			field: "db",
			value: testDSN + " sslmode=wrong",
			valid: false,
		},
		"http": {
			field: "accrual",
			value: "http://localhost:8081",
			valid: true,
		},
		"https slash": {
			field: "accrual",
			value: "https://example.org/",
			valid: true,
		},
		"ipv6 url": {
			field: "accrual",
			value: "http://[::1]:8081/",
			valid: true,
		},
		"relative": {
			field: "accrual",
			value: "localhost:8081",
			valid: false,
		},
		"no host": {
			field: "accrual",
			value: "http:///",
			valid: false,
		},
		"path": {
			field: "accrual",
			value: "http://localhost/api",
			valid: false,
		},
		"query": {
			field: "accrual",
			value: "http://localhost/?x=1",
			valid: false,
		},
		"empty query": {
			field: "accrual",
			value: "http://localhost?",
			valid: false,
		},
		"fragment": {
			field: "accrual",
			value: "http://localhost/#test-secret",
			valid: false,
		},
		"empty fragment": {
			field: "accrual",
			value: "http://localhost#",
			valid: false,
		},
		"credentials": {
			field: "accrual",
			value: "http://u:test-secret@localhost",
			valid: false,
		},
		"empty port": {
			field: "accrual",
			value: "http://localhost:",
			valid: false,
		},
		"zero port": {
			field: "accrual",
			value: "http://localhost:0",
			valid: false,
		},
		"large port": {
			field: "accrual",
			value: "http://localhost:65536",
			valid: false,
		},
		"bad port": {
			field: "accrual",
			value: "http://localhost:abc",
			valid: false,
		},
		"invalid ipv6": {
			field: "accrual",
			value: "http://[xyz]",
			valid: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := Config{
				RunAddress:           ":8080",
				DatabaseURI:          testDSN,
				AccrualSystemAddress: "http://localhost:8081",
			}

			switch tc.field {
			case "run":
				cfg.RunAddress = tc.value
			case "db":
				cfg.DatabaseURI = tc.value
			case "accrual":
				cfg.AccrualSystemAddress = tc.value
			}

			err := NewValidator().Validate(&cfg)

			if !tc.valid {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "test-secret")

				return
			}

			require.NoError(t, err)
		})
	}
}

func TestValidatorErrorReasons(t *testing.T) {
	testCases := map[string]struct {
		field string
		value string
		want  error
	}{
		"empty run": {
			field: "run",
			value: "",
			want:  ErrRunAddressEmpty,
		},
		"run whitespace": {
			field: "run",
			value: " :80",
			want:  ErrRunAddressWhitespace,
		},
		"run syntax": {
			field: "run",
			value: "http://localhost:80",
			want:  ErrRunAddressFormat,
		},
		"run IP": {
			field: "run",
			value: "[xyz]:80",
			want:  ErrRunAddressInvalidIP,
		},
		"port empty": {
			field: "run",
			value: "localhost:",
			want:  ErrPortEmpty,
		},
		"port text": {
			field: "run",
			value: "localhost:x",
			want:  ErrPortNotNumeric,
		},
		"port range": {
			field: "run",
			value: "localhost:65536",
			want:  ErrPortOutOfRange,
		},
		"port overflow": {
			field: "run",
			value: "localhost:999999999999999999999999",
			want:  ErrPortOutOfRange,
		},
		"empty db": {
			field: "db",
			value: " ",
			want:  ErrDatabaseURIEmpty,
		},
		"db scheme": {
			field: "db",
			value: "https://localhost",
			want:  ErrDatabaseSchemeUnsupported,
		},
		"db syntax": {
			field: "db",
			value: "password='secret",
			want:  ErrDatabaseURIInvalid,
		},
		"db sslmode": {
			field: "db",
			value: testDSN + " sslmode=bad",
			want:  ErrDatabaseURIInvalid,
		},
		"empty accrual": {
			field: "accrual",
			value: "",
			want:  ErrAccrualAddressEmpty,
		},
		"accrual syntax": {
			field: "accrual",
			value: "http://secret%zz",
			want:  ErrAccrualAddressInvalid,
		},
		"accrual scheme": {
			field: "accrual",
			value: "ftp://localhost",
			want:  ErrAccrualScheme,
		},
		"accrual host": {
			field: "accrual",
			value: "http:///",
			want:  ErrAccrualHostEmpty,
		},
		"accrual credentials": {
			field: "accrual",
			value: "http://u:secret@localhost",
			want:  ErrAccrualCredentials,
		},
		"accrual path": {
			field: "accrual",
			value: "http://localhost/api",
			want:  ErrAccrualPath,
		},
		"accrual query": {
			field: "accrual",
			value: "http://localhost?",
			want:  ErrAccrualQuery,
		},
		"accrual fragment": {
			field: "accrual",
			value: "http://localhost#",
			want:  ErrAccrualFragment,
		},
		"accrual whitespace": {
			field: "accrual",
			value: " http://localhost",
			want:  ErrAccrualWhitespace,
		},
		"accrual IP": {
			field: "accrual",
			value: "http://[xyz]",
			want:  ErrAccrualAddressInvalid,
		},
		"accrual port": {
			field: "accrual",
			value: "http://localhost:",
			want:  ErrPortEmpty,
		},
		"accrual port range": {
			field: "accrual",
			value: "http://localhost:65536",
			want:  ErrPortOutOfRange,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{
				RunAddress:           ":8080",
				DatabaseURI:          testDSN,
				AccrualSystemAddress: "http://localhost",
			}

			switch tc.field {
			case "run":
				cfg.RunAddress = tc.value
			case "db":
				cfg.DatabaseURI = tc.value
			case "accrual":
				cfg.AccrualSystemAddress = tc.value
			}

			err := NewValidator().Validate(cfg)
			require.ErrorIs(t, err, tc.want)
			require.NotContains(t, err.Error(), "secret")
		})
	}

	require.ErrorIs(t, NewValidator().Validate(nil), ErrConfigNil)
	cfg := &Config{RunAddress: ":65536"}
	err := NewValidator().Validate(cfg)
	require.EqualError(t, err, "validate run address: validate port: port must be between 1 and 65535")
}

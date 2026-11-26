package config_test

import (
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/config"

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
			cfg := config.Config{
				RunAddress:           ":8080",
				DatabaseURI:          testDSN,
				JWTSecret:            "12345678901234567890123456789012",
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

			err := config.NewValidator().Validate(&cfg)

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
			want:  config.ErrRunAddressEmpty,
		},
		"run whitespace": {
			field: "run",
			value: " :80",
			want:  config.ErrRunAddressWhitespace,
		},
		"run syntax": {
			field: "run",
			value: "http://localhost:80",
			want:  config.ErrRunAddressFormat,
		},
		"run IP": {
			field: "run",
			value: "[xyz]:80",
			want:  config.ErrRunAddressInvalidIP,
		},
		"port empty": {
			field: "run",
			value: "localhost:",
			want:  config.ErrPortEmpty,
		},
		"port text": {
			field: "run",
			value: "localhost:x",
			want:  config.ErrPortNotNumeric,
		},
		"port range": {
			field: "run",
			value: "localhost:65536",
			want:  config.ErrPortOutOfRange,
		},
		"port overflow": {
			field: "run",
			value: "localhost:999999999999999999999999",
			want:  config.ErrPortOutOfRange,
		},
		"empty db": {
			field: "db",
			value: " ",
			want:  config.ErrDatabaseURIEmpty,
		},
		"db scheme": {
			field: "db",
			value: "https://localhost",
			want:  config.ErrDatabaseSchemeUnsupported,
		},
		"db syntax": {
			field: "db",
			value: "password='secret",
			want:  config.ErrDatabaseURIInvalid,
		},
		"db sslmode": {
			field: "db",
			value: testDSN + " sslmode=bad",
			want:  config.ErrDatabaseURIInvalid,
		},
		"empty accrual": {
			field: "accrual",
			value: "",
			want:  config.ErrAccrualAddressEmpty,
		},
		"accrual syntax": {
			field: "accrual",
			value: "http://secret%zz",
			want:  config.ErrAccrualAddressInvalid,
		},
		"accrual scheme": {
			field: "accrual",
			value: "ftp://localhost",
			want:  config.ErrAccrualScheme,
		},
		"accrual host": {
			field: "accrual",
			value: "http:///",
			want:  config.ErrAccrualHostEmpty,
		},
		"accrual credentials": {
			field: "accrual",
			value: "http://u:secret@localhost",
			want:  config.ErrAccrualCredentials,
		},
		"accrual path": {
			field: "accrual",
			value: "http://localhost/api",
			want:  config.ErrAccrualPath,
		},
		"accrual query": {
			field: "accrual",
			value: "http://localhost?",
			want:  config.ErrAccrualQuery,
		},
		"accrual fragment": {
			field: "accrual",
			value: "http://localhost#",
			want:  config.ErrAccrualFragment,
		},
		"accrual whitespace": {
			field: "accrual",
			value: " http://localhost",
			want:  config.ErrAccrualWhitespace,
		},
		"accrual IP": {
			field: "accrual",
			value: "http://[xyz]",
			want:  config.ErrAccrualAddressInvalid,
		},
		"accrual port": {
			field: "accrual",
			value: "http://localhost:",
			want:  config.ErrPortEmpty,
		},
		"accrual port range": {
			field: "accrual",
			value: "http://localhost:65536",
			want:  config.ErrPortOutOfRange,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{
				RunAddress:           ":8080",
				DatabaseURI:          testDSN,
				JWTSecret:            "12345678901234567890123456789012",
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

			err := config.NewValidator().Validate(cfg)
			require.ErrorIs(t, err, tc.want)
			require.NotContains(t, err.Error(), "secret")
		})
	}

	require.ErrorIs(t, config.NewValidator().Validate(nil), config.ErrConfigNil)
	cfg := &config.Config{RunAddress: ":65536"}
	err := config.NewValidator().Validate(cfg)
	require.EqualError(t, err, "validate run address: validate port: port must be between 1 and 65535")
}

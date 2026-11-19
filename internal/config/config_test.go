package config_test

import (
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/config"

	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	testCases := map[string]struct {
		environment map[string]string
		args        []string
		wantAddress string
		wantError   bool
	}{
		"defaults": {
			environment: map[string]string{
				"DATABASE_URI":           testDSN,
				"ACCRUAL_SYSTEM_ADDRESS": "http://localhost:8081/",
			},
			wantAddress: ":8080",
		},
		"flags only": {
			args:        []string{"-a", ":8090", "-d", testDSN, "-r", "http://localhost:8081/"},
			wantAddress: ":8090",
		},
		"environment address": {
			environment: map[string]string{
				"RUN_ADDRESS":            ":8091",
				"DATABASE_URI":           testDSN,
				"ACCRUAL_SYSTEM_ADDRESS": "http://localhost:8081",
			},
			wantAddress: ":8091",
		},
		"flags override invalid environment": {
			environment: map[string]string{
				"RUN_ADDRESS":            "bad",
				"DATABASE_URI":           "bad",
				"ACCRUAL_SYSTEM_ADDRESS": "bad",
			},
			args:        []string{"-a", ":8090", "-d", testDSN, "-r", "http://localhost:8081"},
			wantAddress: ":8090",
		},
		"empty flag": {
			environment: map[string]string{
				"RUN_ADDRESS":            ":8091",
				"DATABASE_URI":           testDSN,
				"ACCRUAL_SYSTEM_ADDRESS": "http://localhost:8081",
			},
			args:      []string{"-a="},
			wantError: true,
		},
		"empty environment": {
			environment: map[string]string{
				"RUN_ADDRESS":            "",
				"DATABASE_URI":           testDSN,
				"ACCRUAL_SYSTEM_ADDRESS": "http://localhost:8081",
			},
			wantError: true,
		},
		"missing database": {
			args:      []string{"-r", "http://localhost:8081"},
			wantError: true,
		},
		"missing accrual": {
			args:      []string{"-d", testDSN},
			wantError: true,
		},
		"positional": {
			args:      []string{"test-secret"},
			wantError: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			setConfigProcess(t, tc.environment, tc.args)

			cfg, err := config.LoadConfig()
			if tc.wantError {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "test-secret")

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantAddress, cfg.RunAddress)
			require.Equal(t, testDSN, cfg.DatabaseURI)
			require.Equal(t, "http://localhost:8081", cfg.AccrualSystemAddress)
		})
	}
}

func setConfigProcess(t *testing.T, environment map[string]string, args []string) {
	t.Helper()

	previousArgs := os.Args
	previousFlags := flag.CommandLine
	flag.CommandLine = flag.NewFlagSet("gophermart", flag.ExitOnError)
	t.Cleanup(func() {
		os.Args = previousArgs
		flag.CommandLine = previousFlags
	})

	os.Args = append([]string{"gophermart"}, args...)

	for _, key := range []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS", "JWT_SECRET"} {
		t.Setenv(key, "")
		if value, ok := environment[key]; ok {
			require.NoError(t, os.Setenv(key, value))

			continue
		}

		require.NoError(t, os.Unsetenv(key))
	}
}

func TestLoadConfigJWTSecret(t *testing.T) {
	tests := map[string]struct {
		provided  bool
		secret    string
		wantError bool
	}{
		"absent": {},
		"empty": {
			provided: true,
		},
		"31 bytes": {
			provided:  true,
			secret:    strings.Repeat("a", 31),
			wantError: true,
		},
		"32 bytes": {
			provided: true,
			secret:   strings.Repeat("a", 32),
		},
		"multibyte": {
			provided: true,
			secret:   strings.Repeat("я", 16),
		},
		"spaces preserved": {
			provided: true,
			secret:   strings.Repeat(" ", 32),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			environment := map[string]string{
				"DATABASE_URI":           testDSN,
				"ACCRUAL_SYSTEM_ADDRESS": "http://localhost:8081",
			}
			if tc.provided {
				environment["JWT_SECRET"] = tc.secret
			}
			setConfigProcess(t, environment, nil)

			cfg, err := config.LoadConfig()
			if tc.wantError {
				require.ErrorIs(t, err, config.ErrJWTSecretTooShort)
				if tc.secret != "" {
					require.NotContains(t, err.Error(), tc.secret)
				}
				return
			}

			require.NoError(t, err)
			if tc.secret == "" {
				require.Len(t, cfg.JWTSecret, config.MinJWTSecretBytes)
				setConfigProcess(t, environment, nil)
				next, err := config.LoadConfig()
				require.NoError(t, err)
				require.NotEqual(t, cfg.JWTSecret, next.JWTSecret)
				return
			}

			require.Equal(t, tc.secret, cfg.JWTSecret)
		})
	}
}

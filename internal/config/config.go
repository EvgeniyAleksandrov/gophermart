package config

import (
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/caarlos0/env/v11"
)

// MinJWTSecretBytes is the minimum length of a JWT signing key in bytes.
const MinJWTSecretBytes = 32

// Config contains application settings. DatabaseURI and JWTSecret contain secrets.
type Config struct {
	RunAddress           string `env:"RUN_ADDRESS"`
	DatabaseURI          string `env:"DATABASE_URI"`
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	JWTSecret            string `env:"JWT_SECRET"`
}

// LoadConfig reads process environment and arguments, applies explicit flags over
// environment values, prepares a random JWT secret when empty, and validates the result. Call once at process startup.
// The global flag parser handles help and exits on malformed flags before returning.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, errors.New("parse environment: invalid configuration")
	}

	if _, provided := os.LookupEnv("RUN_ADDRESS"); !provided {
		cfg.RunAddress = ":8080"
	}

	// Bind using the selected environment values, so omitted flags preserve them.
	flag.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "HTTP listen address (RUN_ADDRESS)")
	flag.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "PostgreSQL DSN (DATABASE_URI, required)")
	flag.StringVar(
		&cfg.AccrualSystemAddress,
		"r",
		cfg.AccrualSystemAddress,
		"accrual base URL (ACCRUAL_SYSTEM_ADDRESS, required)",
	)

	flag.Parse()

	if flag.NArg() != 0 {
		return nil, errors.New("parse flags: positional arguments are not supported")
	}

	if cfg.JWTSecret == "" {
		key := make([]byte, MinJWTSecretBytes)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("generate JWT secret: %w", err)
		}

		cfg.JWTSecret = string(key)
	}

	if err := NewValidator().Validate(cfg); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	cfg.AccrualSystemAddress = strings.TrimSuffix(cfg.AccrualSystemAddress, "/")

	return cfg, nil
}

package auth

import (
	"errors"
	"strconv"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = time.Hour

// ErrInvalidToken reports a rejected token without exposing its contents.
var ErrInvalidToken = errors.New("invalid token")

// Clock supplies the current time for issuing and validating tokens.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the process wall clock.
type SystemClock struct{}

// NewSystemClock constructs a wall clock.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now returns the current time.
func (c *SystemClock) Now() time.Time {
	return time.Now()
}

// TokenManager issues and verifies HS256 tokens using an immutable signing key.
type TokenManager struct {
	key    []byte
	clock  Clock
	parser *jwt.Parser
}

// NewTokenManager copies the key and configures strict algorithm and time validation.
func NewTokenManager(key []byte, clock Clock) (*TokenManager, error) {
	if len(key) < config.MinJWTSecretBytes {
		return nil, config.ErrJWTSecretTooShort
	}

	return &TokenManager{
		key:   append([]byte(nil), key...),
		clock: clock,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
			jwt.WithTimeFunc(clock.Now),
		),
	}, nil
}

// Issue signs only the user ID and issuance/expiration times, with a one-hour TTL.
func (m *TokenManager) Issue(userID int64) (string, error) {
	if userID <= 0 {
		return "", ErrInvalidToken
	}

	now := m.clock.Now()
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.key)
}

// Verify checks the signature and required claims without accessing user storage.
func (m *TokenManager) Verify(raw string) (int64, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := m.parser.ParseWithClaims(raw, claims, m.signingKey)
	if err != nil || !token.Valid {
		return 0, ErrInvalidToken
	}

	if claims.IssuedAt == nil || claims.ExpiresAt == nil || !claims.ExpiresAt.After(claims.IssuedAt.Time) {
		return 0, ErrInvalidToken
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID <= 0 {
		return 0, ErrInvalidToken
	}

	return userID, nil
}

func (m *TokenManager) signingKey(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodHS256 {
		return nil, ErrInvalidToken
	}

	return m.key, nil
}

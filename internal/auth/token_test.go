package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type fixedClock struct {
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	return c.now
}

func TestTokenManagerIssue(t *testing.T) {
	clock := &fixedClock{now: time.Unix(1800000000, 123456789)}
	key := []byte(strings.Repeat("k", 32))
	manager, err := NewTokenManager(key, clock)
	require.NoError(t, err)
	key[0] = 'x'

	raw, err := manager.Issue(42)
	require.NoError(t, err)
	id, err := manager.Verify(raw)
	require.NoError(t, err)
	require.Equal(t, int64(42), id)

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) {
		return []byte(strings.Repeat("k", 32)), nil
	}, jwt.WithTimeFunc(clock.Now))
	require.NoError(t, err)
	require.Len(t, claims, 3)
	require.Equal(t, "42", claims["sub"])
	require.Equal(t, float64(1800000000), claims["iat"])
	require.Equal(t, float64(1800003600), claims["exp"])

	clock.now = time.Unix(1800003599, 999999999)
	_, err = manager.Verify(raw)
	require.NoError(t, err)
	clock.now = time.Unix(1800003600, 0)
	_, err = manager.Verify(raw)
	require.ErrorIs(t, err, ErrInvalidToken)

	for _, id := range []int64{0, -1} {
		token, err := manager.Issue(id)
		require.ErrorIs(t, err, ErrInvalidToken)
		require.Empty(t, token)
	}

	_, err = NewTokenManager([]byte("short"), clock)
	require.Error(t, err)
	require.WithinDuration(t, time.Now(), NewSystemClock().Now(), time.Second)
}

func TestTokenManagerVerify(t *testing.T) {
	now := time.Unix(1800000000, 0)
	clock := &fixedClock{now: now}
	key := []byte(strings.Repeat("k", 32))
	manager, err := NewTokenManager(key, clock)
	require.NoError(t, err)

	tests := map[string]struct {
		change func(jwt.MapClaims)
		method jwt.SigningMethod
		key    any
		raw    string
		valid  bool
	}{
		"valid": {
			valid: true,
		},
		"missing sub": {
			change: func(c jwt.MapClaims) { delete(c, "sub") },
		},
		"missing iat": {
			change: func(c jwt.MapClaims) { delete(c, "iat") },
		},
		"missing exp": {
			change: func(c jwt.MapClaims) { delete(c, "exp") },
		},
		"future iat": {
			change: func(c jwt.MapClaims) { c["iat"] = now.Add(time.Second).Unix() },
		},
		"expired": {
			change: func(c jwt.MapClaims) { c["exp"] = now.Unix() },
		},
		"exp equals iat": {
			change: func(c jwt.MapClaims) { c["exp"] = c["iat"] },
		},
		"exp before iat": {
			change: func(c jwt.MapClaims) { c["exp"] = now.Add(-time.Hour).Unix() },
		},
		"zero id": {
			change: func(c jwt.MapClaims) { c["sub"] = "0" },
		},
		"negative id": {
			change: func(c jwt.MapClaims) { c["sub"] = "-1" },
		},
		"overflow id": {
			change: func(c jwt.MapClaims) { c["sub"] = "9223372036854775808" },
		},
		"not decimal id": {
			change: func(c jwt.MapClaims) { c["sub"] = "user" },
		},
		"numeric sub": {
			change: func(c jwt.MapClaims) { c["sub"] = 42 },
		},
		"string exp": {
			change: func(c jwt.MapClaims) { c["exp"] = "tomorrow" },
		},
		"null iat": {
			change: func(c jwt.MapClaims) { c["iat"] = nil },
		},
		"other key": {
			key: []byte(strings.Repeat("x", 32)),
		},
		"HS384": {
			method: jwt.SigningMethodHS384,
		},
		"HS512": {
			method: jwt.SigningMethodHS512,
		},
		"none": {
			method: jwt.SigningMethodNone,
			key:    jwt.UnsafeAllowNoneSignatureType,
		},
		"malformed": {
			raw: "not.a.jwt",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			claims := jwt.MapClaims{
				"sub": "42",
				"iat": now.Unix(),
				"exp": now.Add(time.Hour).Unix(),
			}
			if tc.change != nil {
				tc.change(claims)
			}

			method := tc.method
			if method == nil {
				method = jwt.SigningMethodHS256
			}

			signingKey := tc.key
			if signingKey == nil {
				signingKey = key
			}

			raw := tc.raw
			if raw == "" {
				raw, err = jwt.NewWithClaims(method, claims).SignedString(signingKey)
				require.NoError(t, err)
			}

			id, err := manager.Verify(raw)
			if tc.valid {
				require.NoError(t, err)
				require.Equal(t, int64(42), id)
				return
			}

			require.ErrorIs(t, err, ErrInvalidToken)
			require.Zero(t, id)
			require.NotContains(t, err.Error(), raw)
		})
	}

	raw, err := manager.Issue(42)
	require.NoError(t, err)
	parts := strings.Split(raw, ".")
	parts[2] = strings.Repeat("A", len(parts[2]))
	_, err = manager.Verify(strings.Join(parts, "."))
	require.ErrorIs(t, err, ErrInvalidToken)
}

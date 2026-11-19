package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/auth"
	"github.com/EvgeniyAleksandrov/gophermart/internal/middleware"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAuth(t *testing.T) {
	tests := map[string]struct {
		headers    []string
		verify     bool
		verifyErr  error
		authorized bool
	}{
		"absent": {},
		"empty": {
			headers: []string{""},
		},
		"unsupported scheme": {
			headers: []string{"Basic token"},
		},
		"missing token": {
			headers: []string{"Bearer"},
		},
		"extra component": {
			headers: []string{"Bearer token extra"},
		},
		"duplicate header": {
			headers: []string{"Bearer token", "Bearer token"},
		},
		"newline separator": {
			headers: []string{"Bearer\ntoken"},
		},
		"unicode separator": {
			headers: []string{"Bearer\u00a0token"},
		},
		"verification failure": {
			headers:   []string{"Bearer token"},
			verify:    true,
			verifyErr: errors.New("invalid JWT"),
		},
		"valid": {
			headers:    []string{"Bearer token"},
			verify:     true,
			authorized: true,
		},
		"case and tabs": {
			headers:    []string{" \tbEaReR\t token \t"},
			verify:     true,
			authorized: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tokens := NewMockTokenVerifier(gomock.NewController(t))
			if tc.verify {
				tokens.EXPECT().Verify("token").Return(int64(42), tc.verifyErr)
			}

			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			request := httptest.NewRequest(http.MethodGet, "/test-only", nil).WithContext(parent)
			for _, value := range tc.headers {
				request.Header.Add("Authorization", value)
			}
			called := false
			userContext := requestcontext.NewUser()
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				id, ok := userContext.ID(r.Context())
				require.True(t, ok)
				require.Equal(t, int64(42), id)
				require.NotSame(t, request, r)
				cancel()
				require.ErrorIs(t, r.Context().Err(), context.Canceled)
				w.WriteHeader(http.StatusNoContent)
			})

			response := httptest.NewRecorder()
			middleware.NewAuth(tokens).Wrap(next).ServeHTTP(response, request)
			require.Equal(t, tc.authorized, called)
			_, present := userContext.ID(request.Context())
			require.False(t, present)
			require.Empty(t, response.Header().Get("Authorization"))
			if tc.authorized {
				require.Equal(t, http.StatusNoContent, response.Code)
				require.Empty(t, response.Body.String())
				return
			}

			require.Equal(t, http.StatusUnauthorized, response.Code)
			require.Equal(t, "unauthorized\n", response.Body.String())
		})
	}
}

type tokenClock struct {
	now time.Time
}

func (c *tokenClock) Now() time.Time {
	return c.now
}

func TestAuthRealTokens(t *testing.T) {
	now := time.Unix(1800000000, 0)
	clock := &tokenClock{now: now}
	manager, err := auth.NewTokenManager([]byte(strings.Repeat("k", 32)), clock)
	require.NoError(t, err)
	token, err := manager.Issue(42)
	require.NoError(t, err)
	otherManager, err := auth.NewTokenManager([]byte(strings.Repeat("x", 32)), clock)
	require.NoError(t, err)
	forged, err := otherManager.Issue(42)
	require.NoError(t, err)

	tests := map[string]struct {
		token  string
		now    time.Time
		status int
	}{
		"valid": {
			token:  token,
			now:    now,
			status: http.StatusNoContent,
		},
		"expired at boundary": {
			token:  token,
			now:    now.Add(time.Hour),
			status: http.StatusUnauthorized,
		},
		"wrong signature": {
			token:  forged,
			now:    now,
			status: http.StatusUnauthorized,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			clock.now = tc.now
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				id, ok := requestcontext.NewUser().ID(r.Context())
				require.True(t, ok)
				require.Equal(t, int64(42), id)
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/test-only", nil)
			request.Header.Set("Authorization", "Bearer "+tc.token)
			response := httptest.NewRecorder()
			middleware.NewAuth(manager).Wrap(next).ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, tc.status == http.StatusNoContent, called)
		})
	}
}

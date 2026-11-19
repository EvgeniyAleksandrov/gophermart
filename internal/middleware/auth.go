// Package middleware provides HTTP authorization wrappers.
package middleware

import (
	"net/http"
	"strings"

	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=auth.go -destination=auth_mock_test.go -package=middleware_test

// TokenVerifier verifies a JWT and returns the authenticated user ID.
type TokenVerifier interface {
	Verify(raw string) (int64, error)
}

// Auth authorizes requests without querying storage or renewing tokens.
type Auth struct {
	tokens TokenVerifier
}

// NewAuth receives the application's shared token verifier.
func NewAuth(tokens TokenVerifier) *Auth {
	return &Auth{tokens: tokens}
}

// Wrap requires exactly one bearer header and supplies the verified ID downstream.
func (m *Auth) Wrap(next http.Handler) http.Handler {
	return &authorizedHandler{
		tokens: m.tokens,
		next:   next,
	}
}

type authorizedHandler struct {
	tokens      TokenVerifier
	next        http.Handler
	userContext requestcontext.User
}

func (h *authorizedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	parts := strings.FieldsFunc(values[0], func(r rune) bool { return r == ' ' || r == '\t' })
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	id, err := h.tokens.Verify(parts[1])
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	ctx := h.userContext.WithID(r.Context(), id)
	h.next.ServeHTTP(w, r.WithContext(ctx))
}

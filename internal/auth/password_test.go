package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPasswordHasher(t *testing.T) {
	hasher := NewPasswordHasher()
	require.Equal(t, "5f4dcc3b5aa765d61d8327deb882cf99", hasher.Hash("password"))

	tests := map[string]struct {
		password  string
		candidate string
		matches   bool
	}{
		"same": {
			password:  "password",
			candidate: "password",
			matches:   true,
		},
		"wrong": {
			password:  "password",
			candidate: "other",
		},
		"case": {
			password:  "password",
			candidate: "Password",
		},
		"spaces": {
			password:  "password",
			candidate: " password ",
		},
		"unicode": {
			password:  "пароль",
			candidate: "пароль",
			matches:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.matches, hasher.Matches(tc.candidate, hasher.Hash(tc.password)))
		})
	}

	require.False(t, hasher.Matches("password", "invalid hash"))
}

// Package auth provides password hashing and JWT authentication.
package auth

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
)

// PasswordHasher implements the project's explicitly chosen educational MD5 format.
// This unsalted format is not suitable for protecting real-world passwords.
type PasswordHasher struct{}

// NewPasswordHasher constructs a stateless password hasher.
func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{}
}

// Hash returns lowercase MD5 hex of the original password bytes.
func (h *PasswordHasher) Hash(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}

// Matches compares the computed hash with the stored hash in constant time.
func (h *PasswordHasher) Matches(password, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(h.Hash(password)), []byte(hash)) == 1
}

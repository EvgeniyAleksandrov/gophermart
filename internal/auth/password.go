// Package auth provides password hashing and JWT authentication.
package auth

import (
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
)

type PasswordHasher struct {
}

func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{}
}

func (h *PasswordHasher) Hash(password string) string {
	sum := md5.Sum([]byte(password))
	return hex.EncodeToString(sum[:])
}

func (h *PasswordHasher) Matches(password, hash string) bool {
	return subtle.ConstantTimeCompare([]byte(h.Hash(password)), []byte(hash)) == 1
}

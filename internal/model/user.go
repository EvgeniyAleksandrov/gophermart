// Package model defines business data independently of transport and storage.
package model

// User contains persisted credentials. It must never be serialized in an HTTP response.
type User struct {
	ID           int64
	Login        string
	PasswordHash string
}

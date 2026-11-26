// Package model defines business data independently of transport and storage.
package model

type User struct {
	ID           int64
	Login        string
	PasswordHash string
}

// Package dto defines HTTP payloads and their JSON decoding rules.
package dto

import (
	"encoding/json"
	"io"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

// Credentials contains the login and password supplied by an authentication request.
type Credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Decode reads one JSON object and validates required fields without normalizing them.
// Unknown and repeated fields retain encoding/json semantics. The caller limits input size.
func (c *Credentials) Decode(reader io.Reader) error {
	*c = Credentials{}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(c); err != nil {
		return model.ErrInvalidCredentialsInput
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return model.ErrInvalidCredentialsInput
	}

	if c.Login == "" || c.Password == "" {
		return model.ErrInvalidCredentialsInput
	}

	return nil
}

// Package dto defines HTTP payloads and their JSON decoding rules.
package dto

import (
	"encoding/json"
	"io"

	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
)

type Credentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

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

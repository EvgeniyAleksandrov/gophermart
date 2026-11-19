package dto_test

import (
	"io"
	"strings"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/dto"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/stretchr/testify/require"
)

func TestCredentialsDecode(t *testing.T) {
	valid := `{"login":"Alice","password":"secret"}`
	tests := map[string]struct {
		body        string
		contentType string
		login       string
		password    string
	}{
		"valid": {
			body:        valid,
			contentType: "application/json",
			login:       "Alice",
			password:    "secret",
		},
		"absent content type": {
			body:     valid,
			login:    "Alice",
			password: "secret",
		},
		"wrong content type": {
			body:        valid,
			contentType: "text/plain",
			login:       "Alice",
			password:    "secret",
		},
		"unknown field": {
			body:     `{"login":"Alice","password":"secret","unknown":{"nested":[1]}}`,
			login:    "Alice",
			password: "secret",
		},
		"duplicate fields": {
			body:     `{"login":"first","login":"Alice","password":"secret"}`,
			login:    "Alice",
			password: "secret",
		},
		"whitespace strings": {
			body:     `{"login":" ","password":"  "}`,
			login:    " ",
			password: "  ",
		},
		"whitespace tail": {
			body:     valid + " \n\t",
			login:    "Alice",
			password: "secret",
		},
		"empty": {},
		"whitespace only": {
			body: " \n\t",
		},
		"null": {
			body: "null",
		},
		"array": {
			body: "[]",
		},
		"string": {
			body: `"value"`,
		},
		"number": {
			body: "123",
		},
		"missing fields": {
			body: "{}",
		},
		"missing login": {
			body: `{"password":"secret"}`,
		},
		"missing password": {
			body: `{"login":"Alice"}`,
		},
		"empty login": {
			body: `{"login":"","password":"secret"}`,
		},
		"empty password": {
			body: `{"login":"Alice","password":""}`,
		},
		"null login": {
			body: `{"login":null,"password":"secret"}`,
		},
		"null password": {
			body: `{"login":"Alice","password":null}`,
		},
		"numeric login": {
			body: `{"login":42,"password":"secret"}`,
		},
		"boolean password": {
			body: `{"login":"Alice","password":true}`,
		},
		"object password": {
			body: `{"login":"Alice","password":{}}`,
		},
		"unfinished": {
			body: `{"login":"Alice","password":"secret"`,
		},
		"second object": {
			body: valid + `{}`,
		},
		"second null": {
			body: valid + `null`,
		},
		"garbage tail": {
			body: valid + `!`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var credentials dto.Credentials
			err := credentials.Decode(strings.NewReader(tc.body))
			if tc.login == "" {
				require.ErrorIs(t, err, model.ErrInvalidCredentialsInput)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.login, credentials.Login)
			require.Equal(t, tc.password, credentials.Password)
		})
	}
}

func TestCredentialsDecodeReuse(t *testing.T) {
	credentials := dto.Credentials{}
	require.NoError(t, credentials.Decode(strings.NewReader(`{"login":"Alice","password":"secret"}`)))
	require.ErrorIs(t, credentials.Decode(strings.NewReader(`{"login":"Bob"}`)), model.ErrInvalidCredentialsInput)
	require.Empty(t, credentials.Password)
}

type failedReader struct{}

func (r *failedReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestCredentialsDecodeReadFailure(t *testing.T) {
	var credentials dto.Credentials
	require.ErrorIs(t, credentials.Decode(&failedReader{}), model.ErrInvalidCredentialsInput)
}

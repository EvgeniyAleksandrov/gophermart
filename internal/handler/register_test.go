package handler_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/handler"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRegisterInput(t *testing.T) {
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
		"exact body limit": {
			body:     valid + strings.Repeat(" ", 65536-len(valid)),
			login:    "Alice",
			password: "secret",
		},
		"over body limit": {
			body: valid + strings.Repeat(" ", 65537-len(valid)),
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
			service := NewMockAuthService(gomock.NewController(t))
			h := handler.NewRegister(service, zap.NewNop())
			request := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(tc.body))
			request.Header.Set("Content-Type", tc.contentType)
			var operationCtx context.Context
			before := time.Now()
			if tc.login != "" {
				service.EXPECT().Register(gomock.Any(), tc.login, tc.password).DoAndReturn(func(ctx context.Context, _, _ string) (string, error) {
					operationCtx = ctx
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					require.WithinDuration(t, before.Add(25*time.Second), deadline, time.Second)
					require.NoError(t, ctx.Err())
					return "signed-token", nil
				})
			}

			response := httptest.NewRecorder()
			h.ServeHTTP(response, request)
			if tc.login == "" {
				require.Equal(t, http.StatusBadRequest, response.Code)
				require.Equal(t, "invalid request\n", response.Body.String())
				require.Empty(t, response.Header().Get("Authorization"))
				return
			}

			require.Equal(t, http.StatusOK, response.Code)
			require.Empty(t, response.Body.String())
			require.Equal(t, "Bearer signed-token", response.Header().Get("Authorization"))
			require.ErrorIs(t, operationCtx.Err(), context.Canceled)
			require.NoError(t, request.Context().Err())
		})
	}
}

func TestRegisterServiceErrors(t *testing.T) {
	tests := map[string]struct {
		err    error
		status int
		body   string
		logged bool
	}{
		"invalid input": {
			err:    fmt.Errorf("validate: %w", model.ErrInvalidCredentialsInput),
			status: http.StatusBadRequest,
			body:   "invalid request\n",
		},
		"credentials error": {
			err:    fmt.Errorf("authenticate: %w", model.ErrLoginTaken),
			status: http.StatusConflict,
			body:   "login already taken\n",
		},
		"internal error": {
			err:    errors.New("password hash JWT database details"),
			status: http.StatusInternalServerError,
			body:   "internal server error\n",
			logged: true,
		},
		"deadline": {
			err:    context.DeadlineExceeded,
			status: http.StatusInternalServerError,
			body:   "internal server error\n",
			logged: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			service := NewMockAuthService(gomock.NewController(t))
			service.EXPECT().Register(gomock.Any(), "Alice", "secret").Return("must-not-leak", tc.err)
			core, logs := observer.New(zap.ErrorLevel)
			h := handler.NewRegister(service, zap.New(core))
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"login":"Alice","password":"secret"}`))
			response := httptest.NewRecorder()

			h.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code)
			require.Equal(t, tc.body, response.Body.String())
			require.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
			require.Empty(t, response.Header().Get("Authorization"))
			if !tc.logged {
				require.Zero(t, logs.Len())
				return
			}

			require.Equal(t, 1, logs.Len())
			require.Equal(t, "register failed: internal error", logs.All()[0].Message)
			require.Empty(t, logs.All()[0].Context)
		})
	}
}

type registerFailedBody struct{}

func (b *registerFailedBody) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func (b *registerFailedBody) Close() error {
	return nil
}

func TestRegisterReadFailure(t *testing.T) {
	service := NewMockAuthService(gomock.NewController(t))
	h := handler.NewRegister(service, zap.NewNop())
	request := httptest.NewRequest(http.MethodPost, "/", &registerFailedBody{})
	response := httptest.NewRecorder()

	h.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Empty(t, response.Header().Get("Authorization"))
}

func TestRegisterPreserveParentContext(t *testing.T) {
	parent, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
	cancel()
	service := NewMockAuthService(gomock.NewController(t))
	service.EXPECT().Register(gomock.Any(), "Alice", "secret").DoAndReturn(func(ctx context.Context, _, _ string) (string, error) {
		deadline, _ := ctx.Deadline()
		parentDeadline, _ := parent.Deadline()
		require.Equal(t, parentDeadline, deadline)
		require.ErrorIs(t, ctx.Err(), context.Canceled)
		return "", ctx.Err()
	})
	h := handler.NewRegister(service, zap.NewNop())
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"login":"Alice","password":"secret"}`)).WithContext(parent)
	response := httptest.NewRecorder()

	h.ServeHTTP(response, request)
	require.Equal(t, http.StatusInternalServerError, response.Code)
}

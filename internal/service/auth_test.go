package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces/mock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAuthServiceRegister(t *testing.T) {
	internalErr := errors.New("storage or signer failed")
	tests := map[string]struct {
		login      string
		password   string
		storageErr error
		tokenErr   error
		wantErr    error
	}{
		"success": {
			login:    "Alice",
			password: " secret ",
		},
		"case preserved": {
			login:    "alice",
			password: "secret",
		},
		"spaces accepted": {
			login:    " ",
			password: " ",
		},
		"empty login": {
			password: "secret",
			wantErr:  model.ErrInvalidCredentialsInput,
		},
		"empty password": {
			login:   "Alice",
			wantErr: model.ErrInvalidCredentialsInput,
		},
		"login taken": {
			login:      "Alice",
			password:   "secret",
			storageErr: fmt.Errorf("create: %w", model.ErrLoginTaken),
			wantErr:    model.ErrLoginTaken,
		},
		"storage or commit failure": {
			login:      "Alice",
			password:   "secret",
			storageErr: internalErr,
			wantErr:    internalErr,
		},
		"signing failure after commit": {
			login:    "Alice",
			password: "secret",
			tokenErr: internalErr,
			wantErr:  internalErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage := mock.NewMockDBStorage(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			tokens := NewMockTokenIssuer(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tc.login != "" && tc.password != "" {
				hash := hasher.EXPECT().Hash(tc.password).Return("hash")
				create := storage.EXPECT().AddUser(ctx, tc.login, "hash").
					Return(model.User{ID: 42}, tc.storageErr).
					After(hash)
				if tc.storageErr == nil {
					tokens.EXPECT().Issue(int64(42)).Return("token", tc.tokenErr).After(create)
				}
			}

			token, err := service.NewAuthService(storage, hasher, tokens).Register(ctx, tc.login, tc.password)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Empty(t, token)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "token", token)
		})
	}
}

func TestAuthServiceLogin(t *testing.T) {
	internalErr := errors.New("storage or signer failed")
	tests := map[string]struct {
		login      string
		password   string
		storageErr error
		matches    bool
		tokenErr   error
		wantErr    error
	}{
		"success": {
			login:    "Alice",
			password: " secret ",
			matches:  true,
		},
		"case preserved": {
			login:    "alice",
			password: "secret",
			matches:  true,
		},
		"spaces accepted": {
			login:    " ",
			password: " ",
			matches:  true,
		},
		"empty login": {
			password: "secret",
			wantErr:  model.ErrInvalidCredentialsInput,
		},
		"empty password": {
			login:   "Alice",
			wantErr: model.ErrInvalidCredentialsInput,
		},
		"unknown user": {
			login:      "Alice",
			password:   "secret",
			storageErr: fmt.Errorf("find: %w", model.ErrUserNotFound),
			wantErr:    model.ErrInvalidCredentials,
		},
		"wrong password": {
			login:    "Alice",
			password: "wrong",
			wantErr:  model.ErrInvalidCredentials,
		},
		"storage failure": {
			login:      "Alice",
			password:   "secret",
			storageErr: internalErr,
			wantErr:    internalErr,
		},
		"signing failure": {
			login:    "Alice",
			password: "secret",
			matches:  true,
			tokenErr: internalErr,
			wantErr:  internalErr,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage := mock.NewMockDBStorage(ctrl)
			hasher := NewMockPasswordHasher(ctrl)
			tokens := NewMockTokenIssuer(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tc.login != "" && tc.password != "" {
				find := storage.EXPECT().FindUserByLogin(ctx, tc.login).
					Return(model.User{ID: 42, PasswordHash: "hash"}, tc.storageErr)
				if tc.storageErr == nil {
					match := hasher.EXPECT().Matches(tc.password, "hash").Return(tc.matches).After(find)
					if tc.matches {
						tokens.EXPECT().Issue(int64(42)).Return("token", tc.tokenErr).After(match)
					}
				}
			}

			token, err := service.NewAuthService(storage, hasher, tokens).Login(ctx, tc.login, tc.password)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				require.Empty(t, token)
				return
			}

			require.NoError(t, err)
			require.Equal(t, "token", token)
		})
	}
}

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
	"testing/iotest"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/auth"
	"github.com/EvgeniyAleksandrov/gophermart/internal/handler"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces/mock"
	"github.com/EvgeniyAleksandrov/gophermart/internal/middleware"
	"github.com/EvgeniyAleksandrov/gophermart/internal/model"
	"github.com/EvgeniyAleksandrov/gophermart/internal/requestcontext"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestUploadOrderResponses(t *testing.T) {
	tests := map[string]struct {
		created bool
		err     error
		code    int
		body    string
	}{
		"new": {
			created: true,
			code:    http.StatusAccepted,
		},
		"repeat": {
			code: http.StatusOK,
		},
		"empty": {
			err:  model.ErrEmptyOrderNumber,
			code: http.StatusBadRequest,
			body: "invalid request\n",
		},
		"invalid": {
			err:  model.ErrInvalidOrderNumber,
			code: http.StatusUnprocessableEntity,
			body: "invalid order number\n",
		},
		"missing user": {
			err:  model.ErrUserNotFound,
			code: http.StatusUnauthorized,
			body: "unauthorized\n",
		},
		"other owner": {
			err:  model.ErrOrderOwnedByAnotherUser,
			code: http.StatusConflict,
			body: "order already belongs to another user\n",
		},
		"internal": {
			err:  errors.New("secret database detail"),
			code: http.StatusInternalServerError,
			body: "internal server error\n",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewMockOrderService(gomock.NewController(t))
			resultErr := tc.err
			if resultErr != nil {
				resultErr = fmt.Errorf("wrapped: %w", resultErr)
			}

			svc.EXPECT().Upload(gomock.Any(), int64(7), " \n18 ").Return(tc.created, resultErr)
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/user/orders?user_id=999",
				strings.NewReader(" \n18 "),
			)
			req = req.WithContext(requestcontext.NewUser().WithID(req.Context(), 7))
			rec := httptest.NewRecorder()
			handler.NewUploadOrder(svc, zap.NewNop()).ServeHTTP(rec, req)
			require.Equal(t, tc.code, rec.Code)
			require.Equal(t, tc.body, rec.Body.String())
			require.Empty(t, rec.Header().Get("Authorization"))
			if tc.err != nil {
				require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
			}
		})
	}
}

func TestUploadOrderInput(t *testing.T) {
	tests := map[string]struct {
		body        string
		contentType string
		code        int
	}{
		"no content type": {
			body: "18",
			code: http.StatusAccepted,
		},
		"arbitrary content type": {
			body:        " \r\n18\t ",
			contentType: "application/json",
			code:        http.StatusAccepted,
		},
		"zero": {
			body: "0",
			code: http.StatusAccepted,
		},
		"no 64KiB limit": {
			body: "1" + strings.Repeat("0", 65536) + "8",
			code: http.StatusAccepted,
		},
		"empty": {
			code: http.StatusBadRequest,
		},
		"whitespace": {
			body: " \n\t",
			code: http.StatusBadRequest,
		},
		"internal spaces": {
			body: "1 8",
			code: http.StatusUnprocessableEntity,
		},
		"leading zero": {
			body: "018",
			code: http.StatusUnprocessableEntity,
		},
		"bad checksum": {
			body: "19",
			code: http.StatusUnprocessableEntity,
		},
		"unicode": {
			body: "１８",
			code: http.StatusUnprocessableEntity,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := mock.NewMockDBStorage(gomock.NewController(t))
			if tc.code == http.StatusAccepted {
				check := storage.EXPECT().UserExists(gomock.Any(), int64(7)).Return(true, nil)
				storage.EXPECT().AddOrder(gomock.Any(), int64(7), strings.TrimSpace(tc.body)).
					Return(true, nil).
					After(check)
			}

			svc := service.NewOrder(storage, service.NewOrderNumberValidator())
			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			req = req.WithContext(requestcontext.NewUser().WithID(req.Context(), 7))
			rec := httptest.NewRecorder()
			handler.NewUploadOrder(svc, zap.NewNop()).ServeHTTP(rec, req)
			require.Equal(t, tc.code, rec.Code)
		})
	}
}

func TestUploadOrderReadFailure(t *testing.T) {
	svc := NewMockOrderService(gomock.NewController(t))
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/user/orders",
		io.MultiReader(strings.NewReader("18"), iotest.ErrReader(errors.New("read failed"))),
	)
	req = req.WithContext(requestcontext.NewUser().WithID(req.Context(), 7))
	rec := httptest.NewRecorder()
	handler.NewUploadOrder(svc, zap.NewNop()).ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid request\n", rec.Body.String())
}

func TestUploadOrderAuthorization(t *testing.T) {
	tokens, err := auth.NewTokenManager([]byte(strings.Repeat("k", 32)), auth.NewSystemClock())
	require.NoError(t, err)
	valid, err := tokens.Issue(7)
	require.NoError(t, err)
	tests := map[string]struct {
		header string
		code   int
	}{
		"absent": {
			code: http.StatusUnauthorized,
		},
		"invalid": {
			header: "Bearer invalid",
			code:   http.StatusUnauthorized,
		},
		"valid": {
			header: "Bearer " + valid,
			code:   http.StatusAccepted,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewMockOrderService(gomock.NewController(t))
			if tc.code == http.StatusAccepted {
				svc.EXPECT().Upload(gomock.Any(), int64(7), "18").Return(true, nil)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("18"))
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()
			middleware.NewAuth(tokens).Wrap(handler.NewUploadOrder(svc, zap.NewNop())).ServeHTTP(rec, req)
			require.Equal(t, tc.code, rec.Code)
			require.Empty(t, rec.Header().Get("Authorization"))
		})
	}
}

func TestUploadOrderContext(t *testing.T) {
	for name, tc := range map[string]struct {
		early    bool
		canceled bool
	}{
		"budget": {},
		"parent deadline": {
			early: true,
		},
		"parent cancellation": {
			canceled: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			parent := context.Background()
			if tc.early {
				var cancel context.CancelFunc
				parent, cancel = context.WithTimeout(parent, time.Second)
				defer cancel()
			}

			if tc.canceled {
				var cancel context.CancelFunc
				parent, cancel = context.WithCancel(parent)
				cancel()
			}

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("18"))
			req = req.WithContext(requestcontext.NewUser().WithID(parent, 7))
			storage := mock.NewMockDBStorage(gomock.NewController(t))
			var operation context.Context
			before := time.Now()
			check := storage.EXPECT().UserExists(gomock.Any(), int64(7)).
				DoAndReturn(func(ctx context.Context, id int64) (bool, error) {
					operation = ctx
					deadline, ok := ctx.Deadline()
					require.True(t, ok)
					require.WithinRange(t, deadline, before, time.Now().Add(25*time.Second))
					if tc.early {
						expected, _ := parent.Deadline()
						require.Equal(t, expected, deadline)
					}

					if tc.canceled {
						require.ErrorIs(t, ctx.Err(), context.Canceled)
						return false, ctx.Err()
					}

					if !tc.early {
						require.WithinDuration(t, before.Add(25*time.Second), deadline, time.Second)
					}

					return true, nil
				})
			if !tc.canceled {
				storage.EXPECT().AddOrder(gomock.Any(), int64(7), "18").
					DoAndReturn(func(ctx context.Context, id int64, number string) (bool, error) {
						require.Equal(t, operation, ctx)
						return true, nil
					}).After(check)
			}

			rec := httptest.NewRecorder()
			handler.NewUploadOrder(
				service.NewOrder(storage, service.NewOrderNumberValidator()),
				zap.NewNop(),
			).ServeHTTP(rec, req)
			require.ErrorIs(t, operation.Err(), context.Canceled)
			if tc.canceled {
				require.Equal(t, http.StatusInternalServerError, rec.Code)
			} else {
				require.Equal(t, http.StatusAccepted, rec.Code)
			}
		})
	}
}

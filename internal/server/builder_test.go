package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAddHandlerValidation(t *testing.T) {
	testCases := map[string]struct {
		method  string
		path    string
		handler http.Handler
		wantErr error
	}{
		"unsupported method": {
			method:  "CUSTOM",
			path:    "/orders",
			handler: &statusHandler{code: http.StatusOK},
			wantErr: ErrMethodUnsupported,
		},
		"empty method": {
			method:  "",
			path:    "/orders",
			handler: &statusHandler{code: http.StatusOK},
			wantErr: ErrMethodUnsupported,
		},
		"empty path": {
			method:  http.MethodGet,
			path:    "",
			handler: &statusHandler{code: http.StatusOK},
			wantErr: ErrPathEmpty,
		},
		"relative path": {
			method:  http.MethodGet,
			path:    "orders",
			handler: &statusHandler{code: http.StatusOK},
			wantErr: ErrPathFormat,
		},
		"nil handler": {
			method:  http.MethodGet,
			path:    "/orders",
			handler: nil,
			wantErr: ErrHandlerNil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			builder := NewServerBuilder(":8080")

			err := builder.AddHandler(tc.method, tc.path, tc.handler)
			require.ErrorIs(t, err, tc.wantErr)

			httpServer, err := builder.Build()
			require.NoError(t, err)

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/orders", nil)
			httpServer.Handler.ServeHTTP(response, request)

			require.Equal(t, http.StatusNotFound, response.Code)
		})
	}
}

func TestRoutesAndDuplicate(t *testing.T) {
	builder := NewServerBuilder(":8090")
	require.NoError(t, builder.AddHandler(http.MethodGet, "/orders", &statusHandler{code: http.StatusOK}))
	require.NoError(
		t,
		builder.AddHandler(http.MethodPost, "/orders", &statusHandler{code: http.StatusAccepted}),
	)
	require.NoError(
		t,
		builder.AddHandler(http.MethodGet, "/orders", &statusHandler{code: http.StatusNoContent}),
	)

	httpServer, err := builder.Build()
	require.NoError(t, err)

	testCases := map[string]struct {
		method     string
		path       string
		wantStatus int
	}{
		"replacement handler": {
			method:     http.MethodGet,
			path:       "/orders",
			wantStatus: http.StatusNoContent,
		},
		"different method": {
			method:     http.MethodPost,
			path:       "/orders",
			wantStatus: http.StatusAccepted,
		},
		"unregistered method": {
			method:     http.MethodDelete,
			path:       "/orders",
			wantStatus: http.StatusMethodNotAllowed,
		},
		"unknown path": {
			method:     http.MethodGet,
			path:       "/missing",
			wantStatus: http.StatusNotFound,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(tc.method, tc.path, nil)

			httpServer.Handler.ServeHTTP(response, request)

			require.Equal(t, tc.wantStatus, response.Code)
		})
	}

	require.Equal(t, ":8090", httpServer.Addr)
	require.Equal(t, 5*time.Second, httpServer.ReadHeaderTimeout)
	require.Equal(t, 10*time.Second, httpServer.ReadTimeout)
	require.Equal(t, 30*time.Second, httpServer.WriteTimeout)
	require.Equal(t, 60*time.Second, httpServer.IdleTimeout)
}

func TestSupportedMethods(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodConnect,
		http.MethodOptions,
		http.MethodTrace,
		"QUERY",
		"get",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			builder := NewServerBuilder(":8080")

			err := builder.AddHandler(method, "/", &statusHandler{code: http.StatusOK})

			require.NoError(t, err)
		})
	}
}

type statusHandler struct {
	code int
}

func (h *statusHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(h.code)
}

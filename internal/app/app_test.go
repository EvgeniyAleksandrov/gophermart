package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/server"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestServerTimeoutsAndBind(t *testing.T) {
	httpServer, err := server.NewServerBuilder("127.0.0.1:0").Build()
	require.NoError(t, err)

	application := NewApp(httpServer, zap.NewNop(), newIdleStorage(t))
	require.Equal(t, 5*time.Second, httpServer.ReadHeaderTimeout)
	require.Equal(t, 10*time.Second, httpServer.ReadTimeout)
	require.Equal(t, 30*time.Second, httpServer.WriteTimeout)
	require.Equal(t, 60*time.Second, httpServer.IdleTimeout)
	require.Equal(t, 10*time.Second, application.shutdownTimeout)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer listener.Close()

	httpServer.Addr = listener.Addr().String()
	require.Error(t, application.Run(context.Background()))
}

func TestServerShutdown(t *testing.T) {
	testCases := map[string]struct {
		forced bool
	}{
		"graceful": {
			forced: false,
		},
		"forced": {
			forced: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			exited := make(chan struct{})

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(exited)
				close(entered)

				select {
				case <-release:
					_, _ = io.WriteString(w, "completed")
				case <-r.Context().Done():
				}
			})

			httpServer := &http.Server{
				Addr:    "127.0.0.1:0",
				Handler: handler,
			}
			application := NewApp(httpServer, zap.NewNop(), newIdleStorage(t))
			if tc.forced {
				application.shutdownTimeout = 20 * time.Millisecond
			}

			listener, err := net.Listen("tcp", httpServer.Addr)
			require.NoError(t, err)

			ctx, cancel := contextWithCleanup(t)
			done := make(chan error, 1)
			go func() {
				done <- application.serve(ctx, listener)
			}()

			client := &http.Client{Timeout: time.Second}
			response := make(chan string, 1)
			go func() {
				r, err := client.Get("http://" + listener.Addr().String())
				if err != nil {
					response <- "request failed"

					return
				}
				defer r.Body.Close()

				body, _ := io.ReadAll(r.Body)
				response <- string(body)
			}()

			select {
			case <-entered:
			case <-ctx.Done():
				t.Fatal("request did not start")
			}

			cancel()
			if !tc.forced {
				select {
				case err := <-done:
					t.Fatalf("stopped before active request finished: %v", err)
				default:
				}

				close(release)
			}

			select {
			case err := <-done:
				switch tc.forced {
				case true:
					require.ErrorIs(t, err, context.DeadlineExceeded)
				case false:
					require.NoError(t, err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("shutdown stuck")
			}

			body := <-response
			if !tc.forced {
				require.Equal(t, "completed", body)
			}

			select {
			case <-exited:
			case <-time.After(time.Second):
				t.Fatal("handler stuck")
			}
		})
	}
}

func TestUnexpectedServeFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, listener.Close())

	httpServer := &http.Server{Handler: http.NotFoundHandler()}
	application := NewApp(httpServer, zap.NewNop(), newIdleStorage(t))

	require.Error(t, application.serve(context.Background(), listener))
}

func TestAppCancellation(t *testing.T) {
	storage := NewMockStorage(gomock.NewController(t))
	storage.EXPECT().Close().Return(nil)
	// An invalid address proves an already canceled Run does not open a listener.
	httpServer := &http.Server{Addr: "invalid address"}
	application := NewApp(httpServer, zap.NewNop(), storage)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, application.Run(ctx))
}

func TestAppRun(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	httpServer := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NotFoundHandler(),
	}
	application := NewApp(httpServer, zap.New(core), newIdleStorage(t))

	ctx, cancel := contextWithCleanup(t)
	done := make(chan error, 1)
	go func() {
		done <- application.Run(ctx)
	}()
	require.Eventually(t, func() bool {
		return logs.Len() > 0
	}, time.Second, time.Millisecond)

	address, ok := logs.All()[0].ContextMap()["address"].(string)
	require.True(t, ok, "startup log must contain a string address")
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + address + "/unknown")
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)

	cancel()

	require.NoError(t, <-done)
}

func TestNormalServerClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpServer := &http.Server{}
	require.NoError(t, httpServer.Close())

	application := NewApp(httpServer, zap.NewNop(), newIdleStorage(t))

	require.NoError(t, application.serve(context.Background(), listener))
}

func TestAppStorageCloseFailure(t *testing.T) {
	failure := errors.New("close failed")
	storage := NewMockStorage(gomock.NewController(t))
	storage.EXPECT().Close().Return(failure)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := NewApp(&http.Server{}, zap.NewNop(), storage).Run(ctx)
	require.ErrorIs(t, err, failure)
}

func TestAppClosesStorageAfterHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage := NewMockStorage(ctrl)
	entered := make(chan struct{})
	release := make(chan struct{})
	exited := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		close(exited)
	})
	httpServer := &http.Server{Addr: "127.0.0.1:0", Handler: handler}
	core, logs := observer.New(zap.InfoLevel)
	storage.EXPECT().Close().DoAndReturn(func() error {
		select {
		case <-exited:
		default:
			t.Error("storage closed before handler finished")
		}
		return nil
	})
	ctx, cancel := contextWithCleanup(t)
	done := make(chan error, 1)
	go func() { done <- NewApp(httpServer, zap.New(core), storage).Run(ctx) }()
	require.Eventually(t, func() bool { return logs.Len() > 0 }, time.Second, time.Millisecond)
	address, ok := logs.All()[0].ContextMap()["address"].(string)
	require.True(t, ok)
	responseDone := make(chan struct{})
	go func() {
		defer close(responseDone)
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://" + address)
		if err == nil {
			response.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-done:
		t.Fatal("application stopped with an active handler")
	default:
	}
	close(release)
	require.NoError(t, <-done)
	<-responseDone
}

func TestAppClosesStorageOnListenFailure(t *testing.T) {
	storage := NewMockStorage(gomock.NewController(t))
	storage.EXPECT().Close().Return(nil)
	httpServer := &http.Server{Addr: "invalid address"}

	err := NewApp(httpServer, zap.NewNop(), storage).Run(context.Background())
	require.ErrorContains(t, err, "listen HTTP")
}

func contextWithCleanup(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	return ctx, cancel
}

func newIdleStorage(t *testing.T) *MockStorage {
	t.Helper()
	storage := NewMockStorage(gomock.NewController(t))
	storage.EXPECT().Close().Return(nil).AnyTimes()
	return storage
}

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
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestServerTimeoutsAndBind(t *testing.T) {
	httpServer, err := server.NewServerBuilder("127.0.0.1:0").Build()
	require.NoError(t, err)

	application := NewApp(&stubBuilder{server: httpServer}, zap.NewNop())
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
			application := NewApp(&stubBuilder{server: httpServer}, zap.NewNop())
			if tc.forced {
				application.shutdownTimeout = 20 * time.Millisecond
			}

			listener, err := net.Listen("tcp", httpServer.Addr)
			require.NoError(t, err)

			ctx, cancel := contextWithCleanup(t)
			done := make(chan error, 1)
			go func() {
				done <- application.serve(ctx, httpServer, listener)
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
	application := NewApp(&stubBuilder{server: httpServer}, zap.NewNop())

	require.Error(t, application.serve(context.Background(), httpServer, listener))
}

func TestAppBuildFailureAndCancellation(t *testing.T) {
	failure := errors.New("builder failed")
	builder := &stubBuilder{err: failure}
	application := NewApp(builder, zap.NewNop())
	require.ErrorIs(t, application.Run(context.Background()), failure)
	require.Equal(t, 1, builder.calls)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, application.Run(ctx), failure)
	require.Equal(t, 2, builder.calls)

	builder = &stubBuilder{
		server: &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
		},
	}
	application = NewApp(builder, zap.NewNop())

	require.NoError(t, application.Run(ctx))
	require.Equal(t, 1, builder.calls)
}

func TestAppRun(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	builder := &stubBuilder{
		server: &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: http.NotFoundHandler(),
		},
	}
	application := NewApp(builder, zap.New(core))

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
	require.Equal(t, 1, builder.calls)
}

func TestNormalServerClose(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpServer := &http.Server{}
	require.NoError(t, httpServer.Close())

	application := NewApp(&stubBuilder{server: httpServer}, zap.NewNop())

	require.NoError(t, application.serve(context.Background(), httpServer, listener))
}

func contextWithCleanup(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)

	return ctx, cancel
}

type stubBuilder struct {
	server *http.Server
	err    error
	calls  int
}

func (b *stubBuilder) Build() (*http.Server, error) {
	b.calls++

	return b.server, b.err
}

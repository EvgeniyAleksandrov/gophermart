// Package app manages HTTP startup and graceful shutdown.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"go.uber.org/zap"
)

// ServerBuilder creates the HTTP server used by App.Run.
type ServerBuilder interface {
	Build() (*http.Server, error)
}

const shutdownTimeout = 10 * time.Second

// App builds and runs one HTTP server until cancellation or failure.
type App struct {
	builder         ServerBuilder
	logger          interfaces.Logger
	shutdownTimeout time.Duration
}

// NewApp receives the server builder and shared logger without starting the server.
func NewApp(builder ServerBuilder, logger interfaces.Logger) *App {
	return &App{
		builder:         builder,
		logger:          logger,
		shutdownTimeout: shutdownTimeout,
	}
}

// Run builds and serves HTTP once. Cancellation triggers a separate shutdown
// budget; expected server closure returns nil. Build errors take precedence
// even when the supplied context is already canceled.
func (a *App) Run(ctx context.Context) error {
	server, err := a.builder.Build()
	if err != nil {
		return fmt.Errorf("build server: %w", err)
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	return a.serve(ctx, server, listener)
}

func (a *App) serve(ctx context.Context, server *http.Server, listener net.Listener) error {
	done := make(chan error, 1)

	go func() {
		done <- server.Serve(listener)
	}()

	a.logger.Info("HTTP server started", zap.String("address", listener.Addr().String()))

	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("serve HTTP: %w", errors.Join(err, server.Close()))
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	err := server.Shutdown(shutdownCtx)
	if err != nil {
		err = errors.Join(err, server.Close())
	}

	serveErr := <-done
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}

	if err = errors.Join(err, serveErr); err != nil {
		a.logger.Error("HTTP shutdown failed")

		return fmt.Errorf("shutdown HTTP: %w", err)
	}

	return nil
}

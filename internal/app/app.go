// Package app manages HTTP startup, graceful shutdown and storage cleanup.
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

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=app.go -destination=app_mock_test.go -package=app

// Storage is the shutdown contract for the prepared storage owned by App.
type Storage interface {
	Close() error
}

const shutdownTimeout = 10 * time.Second

// App runs one HTTP server until cancellation or failure.
type App struct {
	storage         Storage
	server          *http.Server
	logger          interfaces.Logger
	shutdownTimeout time.Duration
}

// NewApp receives the prepared server, logger and storage without opening resources.
func NewApp(server *http.Server, logger interfaces.Logger, storage Storage) *App {
	return &App{
		storage:         storage,
		server:          server,
		logger:          logger,
		shutdownTimeout: shutdownTimeout,
	}
}

// Run serves the prepared HTTP server and closes storage on every exit path.
// Cancellation triggers a separate shutdown budget.
func (a *App) Run(ctx context.Context) (err error) {
	defer func() {
		if closeErr := a.storage.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close database: %w", closeErr))
		}
	}()

	if ctx.Err() != nil {
		return nil
	}

	listener, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}

	return a.serve(ctx, listener)
}

func (a *App) serve(ctx context.Context, listener net.Listener) error {
	done := make(chan error, 1)

	go func() {
		done <- a.server.Serve(listener)
	}()

	a.logger.Info("HTTP server started", zap.String("address", listener.Addr().String()))

	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("serve HTTP: %w", errors.Join(err, a.server.Close()))
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	err := a.server.Shutdown(shutdownCtx)
	if err != nil {
		err = errors.Join(err, a.server.Close())
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

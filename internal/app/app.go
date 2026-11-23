// Package app manages HTTP and worker startup, shutdown and storage cleanup.
package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"go.uber.org/zap"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -source=app.go -destination=app_mock_test.go -package=app_test

// Storage is the shutdown contract for the prepared storage owned by App.
type Storage interface{ Close() error }

// Worker runs background processing until its context is canceled.
type Worker interface{ Run(context.Context) error }

const shutdownTimeout = 10 * time.Second

// App owns the lifetime of a prepared server, worker and storage.
type App struct {
	storage Storage
	server  *http.Server
	worker  Worker
	logger  interfaces.Logger
}

// NewApp receives ready components without opening resources.
func NewApp(server *http.Server, logger interfaces.Logger, storage Storage, worker Worker) *App {
	return &App{storage: storage, server: server, worker: worker, logger: logger}
}

// Run cancels HTTP operations and the worker before draining and closing storage.
// Even forced connection closure waits for handlers to release storage references.
func (a *App) Run(ctx context.Context) (err error) {
	defer func() {
		if closeErr := a.storage.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close database: %w", closeErr))
		}
	}()
	if ctx.Err() != nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	workerDone := make(chan error, 1)
	go func() { workerDone <- a.worker.Run(runCtx) }()
	defer func() {
		cancel()
		if workerErr := <-workerDone; workerErr != nil && !errors.Is(workerErr, context.Canceled) {
			err = errors.Join(err, fmt.Errorf("stop worker: %w", workerErr))
		}
	}()
	listener, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	a.server.BaseContext = func(net.Listener) context.Context { return runCtx }
	var active sync.WaitGroup
	var gate sync.Mutex
	stopping := false
	handler := a.server.Handler
	if handler == nil {
		handler = http.DefaultServeMux
	}
	a.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gate.Lock()
		if stopping {
			gate.Unlock()
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		active.Add(1)
		gate.Unlock()
		defer active.Done()
		handler.ServeHTTP(w, r)
	})
	done := make(chan error, 1)
	go func() { done <- a.server.Serve(listener) }()
	a.logger.Info("HTTP server started", zap.String("address", listener.Addr().String()))
	var serveErr error
	served := false
	select {
	case serveErr = <-done:
		served = true
	case workerErr := <-workerDone:
		workerDone <- workerErr
	case <-ctx.Done():
	}
	cancel()
	gate.Lock()
	stopping = true
	gate.Unlock()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	shutdownErr := a.server.Shutdown(shutdownCtx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(shutdownErr, a.server.Close())
	}

	if !served {
		serveErr = <-done
	}
	active.Wait()
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}

	if err := errors.Join(serveErr, shutdownErr); err != nil {
		return fmt.Errorf("stop HTTP: %w", err)
	}

	return nil
}

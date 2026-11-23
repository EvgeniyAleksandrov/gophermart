// Package build assembles concrete application dependencies.
package build

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/EvgeniyAleksandrov/gophermart/internal/app"
	"github.com/EvgeniyAleksandrov/gophermart/internal/auth"
	"github.com/EvgeniyAleksandrov/gophermart/internal/client/accrual"
	"github.com/EvgeniyAleksandrov/gophermart/internal/config"
	"github.com/EvgeniyAleksandrov/gophermart/internal/handler"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/middleware"
	"github.com/EvgeniyAleksandrov/gophermart/internal/repository/postgres"
	"github.com/EvgeniyAleksandrov/gophermart/internal/server"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry"
	"github.com/EvgeniyAleksandrov/gophermart/internal/service/retry/strategy"
	"github.com/EvgeniyAleksandrov/gophermart/internal/worker"
	"github.com/EvgeniyAleksandrov/gophermart/migrations"
)

const (
	dbMaxAttempts         = 5
	dbRetryInitialDelay   = time.Second
	dbRetryMaxDelay       = 10 * time.Second
	accrualRequestTimeout = 10 * time.Second
)

// BuildApp assembles an application using validated configuration and a logger.
// It prepares storage before returning; storage setup failures close the pool.
// One signing key and token manager are shared by all authentication requests.
func BuildApp(cfg *config.Config, logger interfaces.Logger) (*app.App, error) {
	if cfg == nil {
		return nil, errors.New("build app: configuration is nil")
	}

	if logger == nil {
		return nil, errors.New("build app: logger is nil")
	}

	tokens, err := auth.NewTokenManager([]byte(cfg.JWTSecret), auth.NewSystemClock())
	if err != nil {
		return nil, fmt.Errorf("create token manager: %w", err)
	}

	serverBuilder := server.NewServerBuilder(cfg.RunAddress)
	httpServer, err := serverBuilder.Build()
	if err != nil {
		return nil, fmt.Errorf("build server: %w", err)
	}

	dbStrategy, err := strategy.NewExponential(dbMaxAttempts, dbRetryInitialDelay, dbRetryMaxDelay, strategy.NewSQLState([]string{"40001", "40P01"}))
	if err != nil {
		return nil, err
	}

	storageBuilder := postgres.NewDBStorageBuilder(cfg.DatabaseURI, migrations.NewSource().FS(), retry.NewRetryer(dbStrategy))
	storage, err := storageBuilder.Build(context.Background())
	if err != nil {
		return nil, fmt.Errorf("build storage: %w", err)
	}

	authService := service.NewAuthService(storage, auth.NewPasswordHasher(), tokens)
	if err := serverBuilder.AddHandler(http.MethodPost, "/api/user/register", handler.NewRegister(authService, logger)); err != nil {
		return nil, errors.Join(fmt.Errorf("register registration handler: %w", err), storage.Close())
	}

	if err := serverBuilder.AddHandler(http.MethodPost, "/api/user/login", handler.NewLogin(authService, logger)); err != nil {
		return nil, errors.Join(fmt.Errorf("register login handler: %w", err), storage.Close())
	}

	orderService := service.NewOrder(storage, service.NewOrderNumberValidator())
	authorization := middleware.NewAuth(tokens)
	if err := serverBuilder.AddHandler(http.MethodPost, "/api/user/orders", authorization.Wrap(handler.NewUploadOrder(orderService, logger))); err != nil {
		return nil, errors.Join(fmt.Errorf("register upload order handler: %w", err), storage.Close())
	}

	if err := serverBuilder.AddHandler(http.MethodGet, "/api/user/orders", authorization.Wrap(handler.NewListOrders(orderService, logger))); err != nil {
		return nil, errors.Join(fmt.Errorf("register list orders handler: %w", err), storage.Close())
	}

	balanceService := service.NewBalance(storage)
	if err := serverBuilder.AddHandler(http.MethodGet, "/api/user/balance", authorization.Wrap(handler.NewBalance(balanceService, logger))); err != nil {
		return nil, errors.Join(fmt.Errorf("register balance handler: %w", err), storage.Close())
	}

	client := accrual.NewClient(cfg.AccrualSystemAddress, &http.Client{Timeout: accrualRequestTimeout}, retry.NewRetryer(strategy.NewRateLimit()))
	processor := service.NewAccrual(storage, client)
	background := worker.NewAccrual(processor, logger)
	return app.NewApp(httpServer, logger, storage, background), nil
}

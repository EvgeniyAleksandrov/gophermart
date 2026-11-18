// Package build assembles concrete application dependencies.
package build

import (
	"errors"

	"github.com/EvgeniyAleksandrov/gophermart/internal/app"
	"github.com/EvgeniyAleksandrov/gophermart/internal/config"
	"github.com/EvgeniyAleksandrov/gophermart/internal/interfaces"
	"github.com/EvgeniyAleksandrov/gophermart/internal/server"
)

// BuildApp assembles an application using validated configuration and a logger.
// Business handlers will be registered here as their stages are implemented.
func BuildApp(cfg *config.Config, logger interfaces.Logger) (*app.App, error) {
	if cfg == nil {
		return nil, errors.New("build app: configuration is nil")
	}

	if logger == nil {
		return nil, errors.New("build app: logger is nil")
	}

	builder := server.NewServerBuilder(cfg.RunAddress)

	return app.NewApp(builder, logger), nil
}

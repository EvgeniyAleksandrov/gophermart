// Command gophermart starts the loyalty service and handles termination signals.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/EvgeniyAleksandrov/gophermart/internal/build"
	"github.com/EvgeniyAleksandrov/gophermart/internal/config"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintln(os.Stderr, "create logger: failed")
		os.Exit(1)
	}

	// Register exit first so subsequent cleanup defers run before os.Exit.
	exitCode := 0
	defer func() {
		os.Exit(exitCode)
	}()
	defer logger.Sync()

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("load config", zap.Error(err))
		exitCode = 1
		return
	}

	application, err := build.BuildApp(cfg, logger)
	if err != nil {
		logger.Error("build app", zap.Error(err))
		exitCode = 1
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := application.Run(ctx); err != nil {
		logger.Error("run app", zap.Error(err))
		exitCode = 1
		return
	}

	logger.Info("application stopped")
}

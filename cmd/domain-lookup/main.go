package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/grffio/stormwall-domain-lookup/internal/app"
	"github.com/grffio/stormwall-domain-lookup/internal/config"
	"github.com/grffio/stormwall-domain-lookup/pkg/logx"
)

const (
	appName        = "stormwall-domain-lookup"
	appDescription = "service for exact domain lookup in Stormwall"
	configPrefix   = "DOMAIN_LOOKUP"
)

const (
	successExitCode = 0
	failureExitCode = 1
)

func main() {
	os.Exit(run())
}

func run() int {
	const op = "cmd.domain_lookup.run"

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(appName, appDescription, configPrefix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load configuration: %v\n", err)
		return failureExitCode
	}

	logger, err := logx.NewLoggerFromMode(cfg.Env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "initialize logger: %v\n", err)
		return failureExitCode
	}

	log := logger.With(logx.String("op", op))
	log.Debug("initializing application",
		logx.String("app", appName),
		logx.String("env", cfg.Env),
	)

	src, err := app.New(cfg, logger)
	if err != nil {
		log.ErrorContext(ctx, "application initialization failed", logx.Error(err))
		return failureExitCode
	}

	if err = src.Run(ctx); err != nil {
		log.ErrorContext(ctx, "application runtime failed", logx.Error(err))
		return failureExitCode
	}

	log.InfoContext(ctx, "application stopped")
	return successExitCode
}

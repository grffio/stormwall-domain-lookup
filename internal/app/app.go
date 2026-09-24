package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/grffio/stormwall-domain-lookup/internal/client/stormwall"
	"github.com/grffio/stormwall-domain-lookup/internal/config"
	"github.com/grffio/stormwall-domain-lookup/internal/server"
	"github.com/grffio/stormwall-domain-lookup/internal/services/domains"
	"github.com/grffio/stormwall-domain-lookup/internal/storage/memory"
	"github.com/grffio/stormwall-domain-lookup/pkg/health"
	"github.com/grffio/stormwall-domain-lookup/pkg/logx"

	httpapi "github.com/grffio/stormwall-domain-lookup/internal/api"
	httpdomains "github.com/grffio/stormwall-domain-lookup/internal/api/http/domains"
	httphealth "github.com/grffio/stormwall-domain-lookup/internal/api/http/health"
)

const (
	defaultHealthCheckTimeout      = 3 * time.Second
	defaultStormwallAttemptTimeout = 15 * time.Second
	defaultLookupTimeout           = 60 * time.Second
	defaultHTTPWriteTimeout        = 65 * time.Second
	defaultShutdownTimeout         = 70 * time.Second
)

// Server is a lifecycle abstraction for long-running network servers.
//
// Start blocks while the server is running.
// Shutdown must stop accepting new work and cause Start to return.
type Server interface {
	Name() string
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// healthMonitor periodically refreshes application health state and provides
// the latest observed snapshot.
type healthMonitor interface {
	Run(ctx context.Context)
}

// App owns application lifecycle and resources.
type App struct {
	servers       []Server
	closers       []io.Closer
	healthMonitor healthMonitor

	logger *slog.Logger
}

// New creates an application with all dependencies wired.
//
//nolint:funlen // Keep dependency wiring in the composition root.
func New(cfg config.Config, logger *slog.Logger) (app *App, err error) {
	var closers []io.Closer

	defer func() {
		if err != nil {
			err = errors.Join(err, closeAll(closers))
		}
	}()

	healthManager, err := health.NewManager(
		health.WithTimeout(defaultHealthCheckTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("create health manager: %w", err)
	}

	store, err := memory.New(memory.WithTTL(cfg.CacheTTL), memory.WithCleanupInterval(cfg.CacheCleanupInterval))
	if err != nil {
		return nil, fmt.Errorf("create memory store: %w", err)
	}
	closers = append(closers, store)

	domainsRepository := memory.NewDomainsRepository(store)

	stormwallClient, err := stormwall.NewClient(
		cfg.Stormwall.BaseURL,
		logger,
		stormwall.WithTimeout(defaultStormwallAttemptTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("create stormwall client: %w", err)
	}

	if registerErr := healthManager.Register("stormwall client", stormwallClient); registerErr != nil {
		return nil, fmt.Errorf("register stormwall health check: %w", registerErr)
	}

	domainsService, err := domains.NewService(domains.ServiceDeps{
		Repository:    domainsRepository,
		Client:        stormwallClient,
		LookupTimeout: defaultLookupTimeout,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("create domains service: %w", err)
	}
	closers = append(closers, domainsService)

	domainsHandler := httpdomains.NewHandler(domainsService)

	healthMonitor, err := health.NewMonitor(healthManager)
	if err != nil {
		return nil, fmt.Errorf("create health monitor: %w", err)
	}
	healthHandler := httphealth.NewHandler(healthMonitor)

	router := httpapi.NewRouter(httpapi.RouterDeps{
		AuthToken:      cfg.Auth.Token,
		DomainsHandler: domainsHandler,
		HealthHandler:  healthHandler,
	}, logger)

	httpServer, err := server.NewHTTP(
		cfg.ListenAddr,
		router,
		logger,
		server.WithWriteTimeout(defaultHTTPWriteTimeout),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP server: %w", err)
	}

	return &App{
		servers:       []Server{httpServer},
		closers:       closers,
		healthMonitor: healthMonitor,

		logger: logger,
	}, nil
}

// Run starts all servers and blocks until shutdown or startup failure.
func (a *App) Run(ctx context.Context) error {
	const op = "app.run"
	log := a.logger.With(logx.String("op", op))

	if len(a.servers) == 0 {
		return errors.New("no servers configured")
	}

	runCtx, runCancel := context.WithCancel(ctx)
	defer runCancel()

	errCh := make(chan error, len(a.servers))

	var wg sync.WaitGroup

	wg.Go(func() {
		a.healthMonitor.Run(runCtx)
	})

	for _, srv := range a.servers {
		wg.Go(func() {
			if err := srv.Start(ctx); err != nil {
				errCh <- fmt.Errorf("run server %q: %w", srv.Name(), err)
				return
			}

			errCh <- fmt.Errorf("server %q stopped unexpectedly", srv.Name())
		})
	}

	var runErr error

	select {
	case <-ctx.Done():
		log.InfoContext(
			context.WithoutCancel(ctx),
			"application shutdown requested",
			logx.String("reason", ctx.Err().Error()),
		)

	case err := <-errCh:
		runErr = err
	}

	runCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		defaultShutdownTimeout,
	)
	defer shutdownCancel()

	if err := a.Shutdown(shutdownCtx); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("shutdown application: %w", err))
	}

	if err := waitComponents(shutdownCtx, &wg); err != nil {
		runErr = errors.Join(runErr, err)
	}

	return runErr
}

func waitComponents(ctx context.Context, wg *sync.WaitGroup) error {
	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil

	case <-ctx.Done():
		return fmt.Errorf("wait for application components to stop: %w", ctx.Err())
	}
}

// Shutdown gracefully stops servers and releases application resources.
func (a *App) Shutdown(ctx context.Context) error {
	var errs []error

	for _, srv := range slices.Backward(a.servers) {
		if err := srv.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("shutdown server %q: %w", srv.Name(), err))
		}
	}

	if err := closeAll(a.closers); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// closeAll closes resources in reverse initialization order.
func closeAll(closers []io.Closer) error {
	var errs []error

	for _, closer := range slices.Backward(closers) {
		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close resource %T: %w", closer, err))
		}
	}

	return errors.Join(errs...)
}

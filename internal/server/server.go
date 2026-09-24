package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/grffio/stormwall-domain-lookup/pkg/logx"
)

const (
	// defaultHTTPReadHeaderTimeout limits the time to read request headers.
	defaultHTTPReadHeaderTimeout = 2 * time.Second
	// defaultHTTPReadTimeout limits the time to read the full request.
	defaultHTTPReadTimeout = 5 * time.Second
	// defaultHTTPWriteTimeout limits the time to write the response.
	defaultHTTPWriteTimeout = 65 * time.Second
	// defaultHTTPIdleTimeout defines keep-alive idle timeout.
	defaultHTTPIdleTimeout = 120 * time.Second
)

// HTTPServer wraps http server with lifecycle methods.
type HTTPServer struct {
	server *http.Server
	logger *slog.Logger
}

// NewHTTP creates an HTTP server.
func NewHTTP(address string, handler http.Handler, logger *slog.Logger, options ...HTTPOption) (*HTTPServer, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, errors.New("http address must not be empty")
	}
	if handler == nil {
		return nil, errors.New("http handler must not be nil")
	}
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	opts := defaultHTTPOptions()

	for _, option := range options {
		if option == nil {
			return nil, errors.New("http option must not be nil")
		}
		option(&opts)
	}

	if err := validateHTTPOptions(opts); err != nil {
		return nil, fmt.Errorf("validate HTTP options: %w", err)
	}

	return &HTTPServer{
		server: &http.Server{
			Addr:              address,
			Handler:           handler,
			ReadHeaderTimeout: opts.readHeaderTimeout,
			ReadTimeout:       opts.readTimeout,
			WriteTimeout:      opts.writeTimeout,
			IdleTimeout:       opts.idleTimeout,
		},
		logger: logger,
	}, nil
}

// Name returns the server name.
func (s *HTTPServer) Name() string {
	return "http"
}

// Start starts serving HTTP requests and blocks until the server stops.
func (s *HTTPServer) Start(ctx context.Context) error {
	const op = "server.start"
	log := s.logger.With(logx.String("op", op))

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", s.server.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Info("HTTP server started", logx.String("addr", listener.Addr().String()))

	if err := s.server.Serve(listener); err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", err)
	}

	return nil
}

// Shutdown gracefully stops the HTTP server within the caller-provided deadline.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	const op = "server.shutdown"
	log := s.logger.With(logx.String("op", op))

	started := time.Now()
	logCtx := context.WithoutCancel(ctx)

	if deadline, ok := ctx.Deadline(); ok {
		log.InfoContext(
			logCtx,
			"HTTP server shutdown started",
			logx.Duration("remaining", time.Until(deadline)),
		)
	} else {
		log.InfoContext(logCtx, "HTTP server shutdown started")
	}

	if err := s.server.Shutdown(ctx); err != nil {
		log.WarnContext(
			logCtx,
			"HTTP graceful shutdown failed; forcing close",
			logx.Duration("duration", time.Since(started)),
			logx.Error(err),
		)

		closeErr := s.server.Close()
		if closeErr != nil {
			return errors.Join(
				fmt.Errorf("graceful shutdown: %w", err),
				fmt.Errorf("force close: %w", closeErr),
			)
		}

		return fmt.Errorf("graceful shutdown: %w", err)
	}

	log.InfoContext(
		logCtx,
		"HTTP server stopped",
		logx.Duration("duration", time.Since(started)),
	)

	return nil
}

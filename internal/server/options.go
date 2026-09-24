package server

import (
	"errors"
	"time"
)

// HTTPOption configures HTTPServer optional runtime behavior.
type HTTPOption func(*httpOptions)

type httpOptions struct {
	readHeaderTimeout time.Duration
	readTimeout       time.Duration
	writeTimeout      time.Duration
	idleTimeout       time.Duration
}

func defaultHTTPOptions() httpOptions {
	return httpOptions{
		readHeaderTimeout: defaultHTTPReadHeaderTimeout,
		readTimeout:       defaultHTTPReadTimeout,
		writeTimeout:      defaultHTTPWriteTimeout,
		idleTimeout:       defaultHTTPIdleTimeout,
	}
}

func validateHTTPOptions(opts httpOptions) error {
	if opts.readHeaderTimeout < 0 {
		return errors.New("read header timeout must not be negative")
	}
	if opts.readTimeout < 0 {
		return errors.New("read timeout must not be negative")
	}
	if opts.writeTimeout < 0 {
		return errors.New("write timeout must not be negative")
	}
	if opts.idleTimeout < 0 {
		return errors.New("idle timeout must not be negative")
	}

	return nil
}

// WithReadHeaderTimeout sets the maximum duration for reading request headers.
// A zero value disables the timeout, matching net/http behavior.
func WithReadHeaderTimeout(timeout time.Duration) HTTPOption {
	return func(opts *httpOptions) { opts.readHeaderTimeout = timeout }
}

// WithReadTimeout sets the maximum duration for reading the whole request.
// A zero value disables the timeout, matching net/http behavior.
func WithReadTimeout(timeout time.Duration) HTTPOption {
	return func(opts *httpOptions) { opts.readTimeout = timeout }
}

// WithWriteTimeout sets the maximum duration for writing the response.
// A zero value disables the timeout, matching net/http behavior.
func WithWriteTimeout(timeout time.Duration) HTTPOption {
	return func(opts *httpOptions) { opts.writeTimeout = timeout }
}

// WithIdleTimeout sets the maximum keep-alive idle duration.
// A zero value disables the timeout, matching net/http behavior.
func WithIdleTimeout(timeout time.Duration) HTTPOption {
	return func(opts *httpOptions) { opts.idleTimeout = timeout }
}

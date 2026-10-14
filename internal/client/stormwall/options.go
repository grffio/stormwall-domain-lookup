package stormwall

import (
	"context"
	"errors"
	"math"
	"net/http"
	"time"
)

const (
	defaultAttemptTimeout   = 15 * time.Second
	defaultRetries          = 2
	defaultBaseBackoff      = 300 * time.Millisecond
	defaultMaxBackoff       = 2 * time.Second
	defaultMaxRetryAfter    = 5 * time.Second
	defaultMaxResponseBytes = 16 << 20
	defaultSearchPageLimit  = 10

	maxRetries = 10
)

// RateLimiter limits outgoing HTTP attempts.
type RateLimiter interface {
	Wait(ctx context.Context) error
}

// Option configures a Client.
type Option func(*config) error

type config struct {
	attemptTimeout   time.Duration
	retries          int
	baseBackoff      time.Duration
	maxBackoff       time.Duration
	maxRetryAfter    time.Duration
	maxResponseBytes int64
	searchPageLimit  int

	transport http.RoundTripper
	limiter   RateLimiter
}

func defaultConfig() config {
	return config{
		attemptTimeout:   defaultAttemptTimeout,
		retries:          defaultRetries,
		baseBackoff:      defaultBaseBackoff,
		maxBackoff:       defaultMaxBackoff,
		maxRetryAfter:    defaultMaxRetryAfter,
		maxResponseBytes: defaultMaxResponseBytes,
		searchPageLimit:  defaultSearchPageLimit,

		transport: http.DefaultTransport,
	}
}

func (c config) validate() error {
	switch {
	case c.attemptTimeout <= 0:
		return errors.New("attempt timeout must be positive")

	case c.retries < 0 || c.retries > maxRetries:
		return errors.New("retries must be between 0 and 10")

	case c.baseBackoff <= 0:
		return errors.New("base backoff must be positive")

	case c.maxBackoff < c.baseBackoff:
		return errors.New("max backoff must be >= base backoff")

	case c.maxRetryAfter <= 0:
		return errors.New("max retry-after must be positive")

	case c.maxResponseBytes <= 0 ||
		c.maxResponseBytes == math.MaxInt64:
		return errors.New("response size limit must be between 1 and max int64 - 1")

	case c.searchPageLimit <= 0:
		return errors.New("search page limit must be positive")

	case c.transport == nil:
		return errors.New("transport must not be nil")
	}

	return nil
}

// WithTimeout sets the timeout for each HTTP attempt, including body reading.
func WithTimeout(timeout time.Duration) Option {
	return func(c *config) error {
		c.attemptTimeout = timeout
		return nil
	}
}

// WithRetries sets the number of retries after the initial attempt.
func WithRetries(retries int) Option {
	return func(c *config) error {
		c.retries = retries
		return nil
	}
}

// WithBackoff sets the exponential backoff bounds.
func WithBackoff(base, maximum time.Duration) Option {
	return func(c *config) error {
		c.baseBackoff = base
		c.maxBackoff = maximum
		return nil
	}
}

// WithMaxRetryAfter limits how long an operation may wait for Retry-After.
// Longer server delays still extend the client's shared cooldown.
func WithMaxRetryAfter(delay time.Duration) Option {
	return func(c *config) error {
		c.maxRetryAfter = delay
		return nil
	}
}

// WithMaxResponseBytes sets the maximum size of a response body.
func WithMaxResponseBytes(limit int64) Option {
	return func(c *config) error {
		c.maxResponseBytes = limit
		return nil
	}
}

// WithSearchPageLimit sets the maximum number of domains requested per page.
func WithSearchPageLimit(limit int) Option {
	return func(c *config) error {
		c.searchPageLimit = limit
		return nil
	}
}

// WithTransport sets the shared HTTP transport. Callers own its lifecycle.
func WithTransport(transport http.RoundTripper) Option {
	return func(c *config) error {
		c.transport = transport
		return nil
	}
}

// WithRateLimiter sets the limiter shared by outgoing attempts.
func WithRateLimiter(limiter RateLimiter) Option {
	return func(c *config) error {
		if limiter == nil {
			return errors.New("rate limiter must not be nil")
		}

		c.limiter = limiter
		return nil
	}
}

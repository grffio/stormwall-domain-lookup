package memory

import (
	"fmt"
	"time"

	"github.com/patrickmn/go-cache"
)

const (
	defaultTTL             = 5 * time.Minute
	defaultCleanupInterval = 1 * time.Minute
)

type options struct {
	ttl             time.Duration
	cleanupInterval time.Duration
}

// Option configures the in-memory store.
type Option func(*options)

func defaultOptions() options {
	return options{
		ttl:             defaultTTL,
		cleanupInterval: defaultCleanupInterval,
	}
}

// WithTTL sets the default expiration for cached items.
// Zero or cache.NoExpiration disables expiration.
func WithTTL(ttl time.Duration) Option {
	return func(o *options) {
		o.ttl = ttl
	}
}

// WithCleanupInterval sets the interval for removing expired items.
// Zero disables background cleanup.
func WithCleanupInterval(interval time.Duration) Option {
	return func(o *options) {
		o.cleanupInterval = interval
	}
}

func (o options) validate() error {
	if o.ttl < 0 && o.ttl != cache.NoExpiration {
		return fmt.Errorf("invalid cache TTL: %s", o.ttl)
	}

	if o.cleanupInterval < 0 {
		return fmt.Errorf(
			"invalid cache cleanup interval: %s",
			o.cleanupInterval,
		)
	}

	return nil
}

package memory

import (
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

// Store manages a shared in-memory cache with TTL expiration.
type Store struct {
	cache *cache.Cache

	stop chan struct{}
	done chan struct{}
	once sync.Once
}

// New creates an in-memory store with the specified options.
func New(opts ...Option) (*Store, error) {
	cfg := defaultOptions()

	for _, opt := range opts {
		if opt == nil {
			return nil, fmt.Errorf("option must not be nil")
		}

		opt(&cfg)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("validate options: %w", err)
	}

	s := &Store{
		cache: cache.New(cfg.ttl, 0),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}

	if cfg.cleanupInterval > 0 {
		go s.runCleanup(cfg.cleanupInterval)
	} else {
		close(s.done)
	}

	return s, nil
}

func (s *Store) runCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)

	defer ticker.Stop()
	defer close(s.done)

	for {
		select {
		case <-ticker.C:
			s.cache.DeleteExpired()

		case <-s.stop:
			return
		}
	}
}

// Close stops background cleanup and clears cached data.
// The store must not be used after Close.
func (s *Store) Close() error {
	s.once.Do(func() {
		close(s.stop)
		<-s.done
		s.cache.Flush()
	})

	return nil
}

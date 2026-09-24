package health

import (
	"fmt"
	"time"
)

const (
	defaultCheckTimeout   = 30 * time.Second
	defaultMaxConcurrency = 32
)

// Option configures Manager.
type Option func(*Manager) error

// WithTimeout sets the timeout for each dependency check.
func WithTimeout(timeout time.Duration) Option {
	return func(m *Manager) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive: %s", timeout)
		}

		m.timeout = timeout
		return nil
	}
}

// WithMaxConcurrency limits how many dependency checks can run at the same time.
func WithMaxConcurrency(n int) Option {
	return func(m *Manager) error {
		if n <= 0 {
			return fmt.Errorf("max concurrency must be positive: %d", n)
		}

		m.maxConcurrency = n
		return nil
	}
}

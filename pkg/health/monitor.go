package health

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const defaultMonitorInterval = 10 * time.Second

// MonitorOption configures Monitor.
type MonitorOption func(*Monitor) error

// WithInterval sets how often Monitor refreshes its snapshot.
func WithInterval(interval time.Duration) MonitorOption {
	return func(m *Monitor) error {
		if interval <= 0 {
			return fmt.Errorf("interval must be positive: %s", interval)
		}

		m.interval = interval
		return nil
	}
}

// Monitor periodically refreshes and stores the latest health snapshot.
type Monitor struct {
	manager  *Manager
	interval time.Duration
	mu       sync.RWMutex
	snapshot Snapshot
}

// NewMonitor constructs a Monitor for the provided Manager.
func NewMonitor(manager *Manager, opts ...MonitorOption) (*Monitor, error) {
	if manager == nil {
		return nil, errors.New("health manager is nil")
	}

	monitor := &Monitor{
		manager:  manager,
		interval: defaultMonitorInterval,
		snapshot: Snapshot{
			State:        StateUnknown,
			CheckedAt:    time.Time{},
			Dependencies: nil,
		},
	}

	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("health monitor option is nil")
		}

		if err := opt(monitor); err != nil {
			return nil, fmt.Errorf("apply monitor option: %w", err)
		}
	}

	return monitor, nil
}

// Run refreshes the health snapshot until ctx is canceled.
func (m *Monitor) Run(ctx context.Context) {
	m.refresh(ctx)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

// Snapshot returns a copy of the latest observed health snapshot.
func (m *Monitor) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := m.snapshot
	snapshot.Dependencies = append([]DependencyStatus(nil), m.snapshot.Dependencies...)

	return snapshot
}

func (m *Monitor) refresh(ctx context.Context) {
	snapshot := m.manager.Snapshot(ctx)

	m.mu.Lock()
	m.snapshot = snapshot
	m.mu.Unlock()
}

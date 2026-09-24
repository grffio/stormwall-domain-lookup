package health

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
)

// Checker reports the health of a dependency.
type Checker interface {
	Check(ctx context.Context) error
}

// CheckerFunc adapts a function to Checker.
type CheckerFunc func(ctx context.Context) error

// Check calls f(ctx).
func (f CheckerFunc) Check(ctx context.Context) error {
	return f(ctx)
}

// Manager checks the health of registered dependencies.
type Manager struct {
	mu             sync.RWMutex
	checkers       map[string]Checker
	timeout        time.Duration
	maxConcurrency int
}

// NewManager constructs a Manager.
func NewManager(opts ...Option) (*Manager, error) {
	m := &Manager{
		checkers:       make(map[string]Checker),
		timeout:        defaultCheckTimeout,
		maxConcurrency: defaultMaxConcurrency,
	}

	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("health option is nil")
		}

		if err := opt(m); err != nil {
			return nil, fmt.Errorf("apply manager option: %w", err)
		}
	}

	return m, nil
}

// Register adds a named dependency checker.
func (m *Manager) Register(name string, checker Checker) error {
	name = normalizeName(name)
	if name == "" {
		return errors.New("dependency name must not be empty")
	}
	if checker == nil {
		return fmt.Errorf("checker for dependency %q is nil", name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.checkers[name]; exists {
		return fmt.Errorf("dependency %q is already registered", name)
	}

	m.checkers[name] = checker
	return nil
}

// MustRegister adds a named dependency checker and panics on failure.
func (m *Manager) MustRegister(name string, checker Checker) {
	if err := m.Register(name, checker); err != nil {
		panic(err)
	}
}

// Unregister removes a dependency checker by name.
func (m *Manager) Unregister(name string) bool {
	name = normalizeName(name)
	if name == "" {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.checkers[name]
	delete(m.checkers, name)
	return exists
}

// Checker returns a registered checker by name.
func (m *Manager) Checker(name string) (Checker, bool) {
	name = normalizeName(name)
	if name == "" {
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	checker, exists := m.checkers[name]
	return checker, exists
}

// Names returns registered dependency names in deterministic order.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return slices.Sorted(maps.Keys(m.checkers))
}

// Snapshot checks all registered dependencies and returns their aggregated state.
func (m *Manager) Snapshot(ctx context.Context) Snapshot {
	checkers, timeout, maxConcurrency := m.configSnapshot()

	if len(checkers) == 0 {
		return Snapshot{
			State:        StateUnknown,
			CheckedAt:    time.Now(),
			Dependencies: nil,
		}
	}

	statuses := runChecks(ctx, checkers, timeout, maxConcurrency)

	return buildSnapshot(statuses)
}

// Check verifies all registered dependencies.
func (m *Manager) Check(ctx context.Context) error {
	snapshot := m.Snapshot(ctx)
	if snapshot.State == StateUnknown {
		return errors.New("health state unknown: no dependencies registered")
	}

	errs := make([]error, 0)
	for _, dep := range snapshot.Dependencies {
		if dep.State == StateUnhealthy {
			errs = append(errs, DependencyError{
				Name: dep.Name,
				Err:  dep.err,
			})
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) configSnapshot() (map[string]Checker, time.Duration, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	checkers := make(map[string]Checker, len(m.checkers))
	maps.Copy(checkers, m.checkers)

	return checkers, m.timeout, m.maxConcurrency
}

func runChecks(
	ctx context.Context,
	checkers map[string]Checker,
	timeout time.Duration,
	maxConcurrency int,
) []DependencyStatus {
	names := slices.Sorted(maps.Keys(checkers))
	statuses := make([]DependencyStatus, len(names))

	sem := make(chan struct{}, maxConcurrency)

	var wg sync.WaitGroup
	for i, name := range names {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			statuses[i] = runCheck(ctx, name, checkers[name], timeout)
		})
	}

	wg.Wait()
	return statuses
}

func runCheck(ctx context.Context, name string, checker Checker, timeout time.Duration) (status DependencyStatus) {
	start := time.Now()

	status = DependencyStatus{
		Name:      name,
		State:     StateHealthy,
		CheckedAt: start,
	}

	defer func() {
		if v := recover(); v != nil {
			status.State = StateUnhealthy
			status.err = fmt.Errorf("checker panic: %v", v)
			status.Error = status.err.Error()
		}

		status.LatencyMS = time.Since(start).Milliseconds()
	}()

	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := checker.Check(checkCtx); err != nil {
		status.State = StateUnhealthy
		status.err = err
		status.Error = err.Error()
	}

	return status
}

func buildSnapshot(statuses []DependencyStatus) Snapshot {
	snapshot := Snapshot{
		State:        StateHealthy,
		CheckedAt:    time.Now(),
		Dependencies: statuses,
	}

	for _, status := range statuses {
		if status.State != StateHealthy {
			snapshot.State = StateUnhealthy
			break
		}
	}

	return snapshot
}

func normalizeName(name string) string {
	return strings.TrimSpace(name)
}

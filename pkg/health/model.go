package health

import "time"

// State describes the health state of a dependency or the whole system.
type State string

const (
	// StateUnknown means the health state has not been determined yet.
	StateUnknown State = "unknown"
	// StateHealthy means all required health checks passed.
	StateHealthy State = "healthy"
	// StateUnhealthy means at least one required health check failed.
	StateUnhealthy State = "unhealthy"
)

// String returns the string representation of State.
func (s State) String() string {
	return string(s)
}

// Valid reports whether s is one of the known health states.
func (s State) Valid() bool {
	return s == StateUnknown || s == StateHealthy || s == StateUnhealthy
}

// Snapshot describes the health state of the system at a point in time.
type Snapshot struct {
	State        State
	CheckedAt    time.Time
	Dependencies []DependencyStatus
}

// DependencyStatus describes the latest health check result for a dependency.
type DependencyStatus struct {
	Name      string
	State     State
	Error     string
	CheckedAt time.Time
	LatencyMS int64

	err error
}

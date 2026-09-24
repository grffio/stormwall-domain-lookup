package health

import "time"

// healthResponse is the public response contract for health endpoints.
type healthResponse struct {
	State        string             `json:"state"`
	CheckedAt    time.Time          `json:"checked_at"`
	Dependencies []healthDependency `json:"dependencies"`
}

// healthDependency is the public response contract for one dependency health check.
type healthDependency struct {
	Name      string    `json:"name"`
	State     string    `json:"state"`
	Error     string    `json:"error,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
	LatencyMS int64     `json:"latency_ms"`
}

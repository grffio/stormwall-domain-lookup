package health

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/grffio/stormwall-domain-lookup/internal/api/http/requestx"
	"github.com/grffio/stormwall-domain-lookup/pkg/health"
	"github.com/grffio/stormwall-domain-lookup/pkg/httpx"
	"github.com/grffio/stormwall-domain-lookup/pkg/logx"
)

// SnapshotProvider provides the latest application health snapshot.
type SnapshotProvider interface {
	Snapshot() health.Snapshot
}

// Handler handles application health endpoints.
type Handler struct {
	monitor SnapshotProvider

	mu            sync.Mutex
	previous      map[string]health.DependencyStatus
	previousState health.State
	observed      bool
}

// NewHandler creates a health handler.
func NewHandler(monitor SnapshotProvider) *Handler {
	return &Handler{
		monitor: monitor,
	}
}

// Livez reports process liveness.
func (h *Handler) Livez(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteText(w, http.StatusOK, "ok\n")
}

// Readyz reports application readiness based on the latest health snapshot.
func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	const op = "api.http.health.readyz"
	log := requestx.LoggerFromRequest(r, op)

	snapshot := h.monitor.Snapshot()

	h.logHealthChanges(r.Context(), snapshot, log)

	statusCode := http.StatusOK
	if snapshot.State != health.StateHealthy {
		statusCode = http.StatusServiceUnavailable
	}

	dependencies := make([]healthDependency, 0, len(snapshot.Dependencies))
	for _, dep := range snapshot.Dependencies {
		dependencies = append(dependencies, healthDependency{
			Name:      dep.Name,
			State:     dep.State.String(),
			Error:     dep.Error,
			CheckedAt: dep.CheckedAt,
			LatencyMS: dep.LatencyMS,
		})
	}

	resp := healthResponse{
		State:        snapshot.State.String(),
		CheckedAt:    snapshot.CheckedAt,
		Dependencies: dependencies,
	}

	if err := httpx.WriteJSON(w, statusCode, resp); err != nil {
		log.ErrorContext(r.Context(),
			"HTTP response encoding failed",
			logx.Error(err),
		)
	}
}

func (h *Handler) logHealthChanges(ctx context.Context, snapshot health.Snapshot, log *slog.Logger) {
	h.mu.Lock()
	defer h.mu.Unlock()

	next := make(map[string]health.DependencyStatus, len(snapshot.Dependencies))

	for _, dep := range snapshot.Dependencies {
		next[dep.Name] = dep

		previous, exists := h.previous[dep.Name]

		if exists && previous.State == dep.State {
			if dep.State != health.StateHealthy &&
				previous.Error != dep.Error {
				log.DebugContext(ctx, "dependency health error changed",
					logx.String("dependency", dep.Name),
					logx.String("state", dep.State.String()),
					logx.String("previous_error", previous.Error),
					logx.String("error", dep.Error),
					logx.Int("latency_ms", dep.LatencyMS),
				)
			}

			continue
		}

		previousState := health.StateUnknown
		if exists {
			previousState = previous.State
		}

		if dep.State == health.StateHealthy {
			if exists {
				log.InfoContext(ctx, "dependency recovered",
					logx.String("dependency", dep.Name),
					logx.String("previous_state", previousState.String()),
					logx.Int("latency_ms", dep.LatencyMS),
				)
			}

			continue
		}

		log.WarnContext(ctx, "dependency health check failed",
			logx.String("dependency", dep.Name),
			logx.String("previous_state", previousState.String()),
			logx.String("state", dep.State.String()),
			logx.String("error", dep.Error),
			logx.Int("latency_ms", dep.LatencyMS),
		)
	}

	if len(snapshot.Dependencies) == 0 &&
		snapshot.State != health.StateHealthy &&
		(!h.observed || h.previousState != snapshot.State) {
		log.WarnContext(ctx, "readiness state unavailable",
			logx.String("state", snapshot.State.String()),
		)
	}

	h.previous = next
	h.previousState = snapshot.State
	h.observed = true
}

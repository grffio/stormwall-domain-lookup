package health_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httphealth "github.com/grffio/stormwall-domain-lookup/internal/api/http/health"
	apphealth "github.com/grffio/stormwall-domain-lookup/pkg/health"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type snapshotProviderStub struct {
	snapshotFn func() apphealth.Snapshot
}

func (s snapshotProviderStub) Snapshot() apphealth.Snapshot {
	if s.snapshotFn == nil {
		panic("unexpected Snapshot call")
	}

	return s.snapshotFn()
}

type readyzResponse struct {
	State        string    `json:"state"`
	CheckedAt    time.Time `json:"checked_at"`
	Dependencies []any     `json:"dependencies"`
}

func TestHandler_Livez(t *testing.T) {
	t.Parallel()

	handler := httphealth.NewHandler(snapshotProviderStub{})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/livez", http.NoBody)
	rec := httptest.NewRecorder()

	handler.Livez(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/plain; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, "ok\n", rec.Body.String())
}

func TestHandler_Readyz(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, time.May, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		snapshot   apphealth.Snapshot
		wantStatus int
	}{
		{
			name: "healthy application",
			snapshot: apphealth.Snapshot{
				State:        apphealth.StateHealthy,
				CheckedAt:    checkedAt,
				Dependencies: nil,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "unhealthy application",
			snapshot: apphealth.Snapshot{
				CheckedAt:    checkedAt,
				Dependencies: nil,
			},
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := httphealth.NewHandler(snapshotProviderStub{
				snapshotFn: func() apphealth.Snapshot {
					return tt.snapshot
				},
			})

			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/readyz", http.NoBody)
			rec := httptest.NewRecorder()

			handler.Readyz(rec, req)

			res := rec.Result()

			assert.Equal(t, tt.wantStatus, res.StatusCode)
			assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))

			var got readyzResponse
			require.NoError(t, json.NewDecoder(res.Body).Decode(&got))

			assert.Equal(t, tt.snapshot.State.String(), got.State)
			assert.Equal(t, checkedAt, got.CheckedAt)
			assert.Empty(t, got.Dependencies)
		})
	}
}

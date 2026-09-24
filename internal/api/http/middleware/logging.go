package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/grffio/stormwall-domain-lookup/pkg/logx"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Logger returns an HTTP middleware that attaches a request-scoped logger to the context
// and emits access log entries for request start and completion.
func Logger(baseLogger *slog.Logger) func(http.Handler) http.Handler {
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			log := baseLogger.With(
				logx.String("request_id", chimw.GetReqID(r.Context())),
				logx.String("method", r.Method),
				logx.String("path", r.URL.Path),
				logx.String("remote_addr", r.RemoteAddr),
			)

			ctx := logx.WithLogger(r.Context(), log)

			log.DebugContext(ctx, "HTTP request started")

			defer func() {
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}

				level := slog.LevelInfo

				if isHealthEndpoint(r.URL.Path) {
					level = slog.LevelDebug
				}

				log.LogAttrs(
					ctx,
					level,
					"HTTP request completed",
					logx.Int("status", status),
					logx.Int("bytes", ww.BytesWritten()),
					logx.Int("duration_ms", time.Since(started).Milliseconds()),
				)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

func isHealthEndpoint(path string) bool {
	return path == "/livez" || path == "/readyz"
}

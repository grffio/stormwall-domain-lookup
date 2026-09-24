package requestx

import (
	"log/slog"
	"net/http"

	"github.com/grffio/stormwall-domain-lookup/pkg/logx"
)

// LoggerFromRequest returns a request-scoped logger enriched with an operation name.
func LoggerFromRequest(r *http.Request, op string) *slog.Logger {
	return logx.LoggerFromContext(r.Context(), nil).With(
		logx.String("op", op),
	)
}

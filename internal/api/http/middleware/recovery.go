package middleware

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/grffio/stormwall-domain-lookup/pkg/logx"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// Recoverer logs handler panics and returns HTTP 500 when possible.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		defer func(ctx context.Context) {
			if recovered := recover(); recovered != nil {
				logx.LoggerFromContext(ctx, nil).ErrorContext(
					ctx,
					"HTTP handler panicked",
					logx.String("op", "api.http.recover"),
					logx.String("panic_type", fmt.Sprintf("%T", recovered)),
					logx.String("panic_value", fmt.Sprint(recovered)),
					logx.String("stack", string(debug.Stack())),
				)

				if ww, ok := w.(chimw.WrapResponseWriter); !ok || ww.Status() == 0 {
					http.Error(
						w,
						http.StatusText(http.StatusInternalServerError),
						http.StatusInternalServerError,
					)
				}
			}
		}(ctx)

		next.ServeHTTP(w, r)
	})
}

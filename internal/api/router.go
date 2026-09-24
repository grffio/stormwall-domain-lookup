package router

import (
	"log/slog"
	"net/http"

	"github.com/grffio/stormwall-domain-lookup/internal/api/http/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// DomainsHandler defines HTTP handlers for domain operations.
type DomainsHandler interface {
	Search(w http.ResponseWriter, r *http.Request)
}

// HealthHandler defines health endpoints required by the router.
type HealthHandler interface {
	Livez(w http.ResponseWriter, r *http.Request)
	Readyz(w http.ResponseWriter, r *http.Request)
}

// RouterDeps contains dependencies required to build the HTTP router.
//
//revive:disable-next-line:exported Public API reads as httpapi.RouterDeps at call sites.
type RouterDeps struct {
	AuthToken      string
	DomainsHandler DomainsHandler
	HealthHandler  HealthHandler
}

// NewRouter wires HTTP routes to handlers and middleware.
func NewRouter(deps RouterDeps, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()

	router.Use(chimw.ClientIPFromRemoteAddr)
	router.Use(chimw.RequestID)
	router.Use(middleware.Logger(logger))
	router.Use(middleware.Recoverer)

	router.Group(func(r chi.Router) {
		r.Get("/livez", deps.HealthHandler.Livez)
		r.Get("/readyz", deps.HealthHandler.Readyz)
	})

	router.Route("/v3", func(r chi.Router) {
		if deps.AuthToken != "" {
			r.Use(middleware.Auth(deps.AuthToken))
		}

		r.Get("/domains", deps.DomainsHandler.Search)
	})

	return router
}

package domains

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/grffio/stormwall-domain-lookup/internal/api/http/requestx"
	domain "github.com/grffio/stormwall-domain-lookup/internal/services/domains"
	"github.com/grffio/stormwall-domain-lookup/pkg/httpx"
	"github.com/grffio/stormwall-domain-lookup/pkg/logx"
)

// Service defines domain use cases required by Handler.
type Service interface {
	FindExact(ctx context.Context, apiKey, serviceID, name string) (domain.Domain, error)
}

// Handler handles domain search requests.
type Handler struct {
	service Service
}

// NewHandler creates a domain handler.
func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

// Search handles domain search requests.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	const op = "api.http.domains.search"
	log := requestx.LoggerFromRequest(r, op)

	apiKey, ok := requestx.APIKeyFromRequest(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "X-API-Key header is required")
		return
	}

	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid query parameters")
		return
	}

	serviceIDs := params["serviceID"]
	if len(serviceIDs) != 1 {
		httpx.WriteError(w, http.StatusBadRequest, "serviceID parameter is required")
		return
	}
	serviceID := serviceIDs[0]

	names := params["name"]
	if len(names) != 1 {
		httpx.WriteError(w, http.StatusBadRequest, "name parameter is required")
		return
	}
	name := names[0]

	log = log.With(
		logx.String("service_id", serviceID),
		logx.String("domain", name),
	)
	log.DebugContext(
		r.Context(),
		"domain search request accepted",
	)

	item, err := h.service.FindExact(r.Context(), apiKey, serviceID, name)
	if err != nil && r.Context().Err() != nil {
		return
	}

	response := searchResponse{
		Status: "ok",
		Payload: searchPayload{
			Limit:   1,
			Results: []domainResponse{},
		},
	}

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidAPIKey):
			httpx.WriteError(w, http.StatusUnauthorized, "invalid X-API-Key header")

		case errors.Is(err, domain.ErrInvalidServiceID):
			httpx.WriteError(w, http.StatusBadRequest, "invalid serviceID parameter")

		case errors.Is(err, domain.ErrInvalidName):
			httpx.WriteError(w, http.StatusBadRequest, "invalid domain name")

		case errors.Is(err, domain.ErrNotFound):
			if writeErr := httpx.WriteJSON(w, http.StatusOK, response); writeErr != nil {
				log.ErrorContext(r.Context(),
					"HTTP response encoding failed",
					logx.Error(writeErr),
				)
			}

		default:
			if writeUpstreamHTTPError(w, err) {
				return
			}

			log.ErrorContext(r.Context(), "domain lookup failed", logx.Error(err))
			httpx.WriteError(w,
				http.StatusInternalServerError,
				http.StatusText(http.StatusInternalServerError),
			)
		}

		return
	}

	response.Payload.Total = 1
	response.Payload.Results = []domainResponse{
		{
			ID:     item.ID,
			Domain: item.Name,
		},
	}

	if err := httpx.WriteJSON(w, http.StatusOK, response); err != nil {
		log.ErrorContext(r.Context(), "HTTP response encoding failed", logx.Error(err))
	}
}

type upstreamHTTPError interface {
	HTTPStatusCode() int
}

func writeUpstreamHTTPError(w http.ResponseWriter, err error) bool {
	var upstreamErr upstreamHTTPError
	if !errors.As(err, &upstreamErr) {
		return false
	}

	switch upstreamErr.HTTPStatusCode() {
	case http.StatusUnauthorized:
		httpx.WriteError(w, http.StatusUnauthorized, "invalid Stormwall API key")
		return true

	case http.StatusForbidden:
		httpx.WriteError(w, http.StatusForbidden, "Stormwall access forbidden")
		return true

	case http.StatusTooManyRequests:
		httpx.WriteError(w, http.StatusTooManyRequests, "Stormwall rate limit exceeded")
		return true

	default:
		return false
	}
}

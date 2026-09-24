package requestx

import "net/http"

const stormwallAPIKeyHeader = "X-API-Key" // #nosec G101 -- Header name, not a credential.

// APIKeyFromRequest returns the single Stormwall API key supplied with the request.
func APIKeyFromRequest(r *http.Request) (string, bool) {
	values := r.Header.Values(stormwallAPIKeyHeader)
	if len(values) != 1 || values[0] == "" {
		return "", false
	}

	return values[0], true
}

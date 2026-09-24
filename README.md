# stormwall-domain-lookup

Small HTTP service for exact domain lookup in Stormwall.

The service accepts a Stormwall API key, service ID, and domain name. It checks the local in-memory cache first and queries the Stormwall API on a cache miss.

Stormwall may return multiple domains for a search query, so the service performs an additional exact match before returning a result.

Concurrent cache misses for the same API key, service ID, and domain are combined into a single upstream request.

## Run

No configuration is required for a basic start:

```bash
go run ./cmd/domain-lookup
```

The service listens on `:8080` by default.

To protect the domain API with a Bearer token:

```bash
export DOMAIN_LOOKUP_AUTH_TOKEN="change-me"

go run ./cmd/domain-lookup
```

The Stormwall API key is not configured globally. Clients provide it with each domain lookup using the `X-API-Key` header.

### Configuration

| Variable                               | Default                             | Description                                                                     |
| -------------------------------------- | ----------------------------------- | ------------------------------------------------------------------------------- |
| `DOMAIN_LOOKUP_ENV`                    | `production`                        | Runtime mode: `local`, `develop`, `production`                                  |
| `DOMAIN_LOOKUP_LISTEN_ADDR`            | `:8080`                             | HTTP listen address                                                             |
| `DOMAIN_LOOKUP_CACHE_TTL`              | `3m`                                | TTL for cached domain records                                                   |
| `DOMAIN_LOOKUP_CACHE_CLEANUP_INTERVAL` | `1m`                                | Interval for removing expired cache records; `0` disables background cleanup    |
| `DOMAIN_LOOKUP_AUTH_TOKEN`             | empty                               | Optional Bearer token protecting `/v3/*`; authentication is disabled when empty |
| `DOMAIN_LOOKUP_STORMWALL_BASE_URL`     | `https://api.stormwall.pro/v3/`     | Stormwall API base URL                                                          |

Configuration can be supplied through environment variables or command-line flags.

## API

### Exact domain lookup

```text
GET /v3/domains?serviceID=<service-id>&name=<domain>
X-API-Key: <stormwall-api-key>
Authorization: Bearer <token>
```

`X-API-Key` is required and is forwarded to Stormwall.

`Authorization` is required only when `DOMAIN_LOOKUP_AUTH_TOKEN` is configured.

Example with API authentication enabled:

```bash
curl \
  -H "Authorization: Bearer ${DOMAIN_LOOKUP_AUTH_TOKEN}" \
  -H "X-API-Key: ${STORMWALL_API_KEY}" \
  "http://localhost:8080/v3/domains?serviceID=123&name=example.com"
```

Example without API authentication:

```bash
curl \
  -H "X-API-Key: ${STORMWALL_API_KEY}" \
  "http://localhost:8080/v3/domains?serviceID=123&name=example.com"
```

### Domain matching

Domain names are normalized before lookup:

* surrounding whitespace is removed;
* the name is converted to lowercase;
* a trailing `.` is removed;
* DNS labels are validated.

For example, `Example.COM.` is looked up as `example.com`.

The normalized domain name must match a domain returned by Stormwall exactly. Partial matches returned by the upstream search are ignored.

### Found

```json
{
  "status": "ok",
  "payload": {
    "total": 1,
    "limit": 1,
    "offset": 0,
    "results": [
      {
        "id": 12345,
        "domain": "example.com"
      }
    ]
  }
}
```

### Not found

A missing domain is not treated as an HTTP error. The service returns `200 OK` with an empty result:

```json
{
  "status": "ok",
  "payload": {
    "total": 0,
    "limit": 1,
    "offset": 0,
    "results": []
  }
}
```

Negative results are not cached.

### Errors

The API may return:

| Status                      | Description                                                                                |
| --------------------------- | ------------------------------------------------------------------------------------------ |
| `400 Bad Request`           | Invalid or missing `serviceID` or invalid domain name                                      |
| `401 Unauthorized`          | Missing/invalid `X-API-Key`, invalid Stormwall API key, or invalid configured Bearer token |
| `403 Forbidden`             | Stormwall rejected access                                                                  |
| `429 Too Many Requests`     | Stormwall rate limit exceeded                                                              |
| `500 Internal Server Error` | Unexpected lookup or upstream error                                                        |

Error responses are returned as plain text.

## Cache

Successful exact domain lookups are stored in an in-memory cache.

The cache key contains:

* Stormwall API key scope;
* service ID;
* normalized domain name.

The raw Stormwall API key is not stored as part of the cache key; a SHA-256 based credential scope is used instead.

This prevents cached results obtained with one Stormwall credential from being reused by another credential.

Default cache TTL is `3m`.

Concurrent cache misses for the same credential scope, service ID, and normalized domain are deduplicated using singleflight, so only one Stormwall request is performed.

## Upstream requests

The service queries:

```text
GET <stormwall-base-url>/domains?serviceId=<service-id>&search=<domain>
X-API-Key: <stormwall-api-key>
Accept: application/json
```

An upstream request attempt has a `15s` timeout.

Retryable failures are retried up to two times after the initial request. Retry handling covers transient network failures and retryable HTTP statuses such as `408`, `500`, `502`, `503`, and `504`.

Stormwall `429 Too Many Requests` responses are also retryable and `Retry-After` is respected within the configured client limits.

The complete shared domain lookup is bounded by a `60s` timeout.

## Health checks

```text
GET /livez
GET /readyz
```

Health endpoints are not protected by Bearer authentication.

### Liveness

```text
GET /livez
```

Returns:

```text
ok
```

with `200 OK` while the HTTP process is running.

### Readiness

```text
GET /readyz
```

Readiness is based on the latest health snapshot for the Stormwall client.

The Stormwall health check verifies direct TCP/TLS connectivity to the configured Stormwall endpoint. It does not perform an authenticated Stormwall API request and does not validate an `X-API-Key`.

A healthy response returns `200 OK`:

```json
{
  "state": "healthy",
  "checked_at": "2026-09-24T09:00:00Z",
  "dependencies": [
    {
      "name": "stormwall client",
      "state": "healthy",
      "checked_at": "2026-09-24T09:00:00Z",
      "latency_ms": 42
    }
  ]
}
```

If the dependency check fails, `/readyz` returns `503 Service Unavailable` with `state` set to `unhealthy` and the dependency error included in the response.

The health snapshot is refreshed periodically; the Stormwall connectivity check has a `3s` timeout.

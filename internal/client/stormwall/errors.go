package stormwall

import (
	"fmt"
	"net/http"
	"time"
)

// StatusError represents an unexpected HTTP status.
type StatusError struct {
	StatusCode int
	Method     string
	Endpoint   string
}

// Error returns the HTTP status error message.
func (e *StatusError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.Endpoint, e.StatusCode)
}

// HTTPStatusCode returns the upstream HTTP status code.
func (e *StatusError) HTTPStatusCode() int {
	return e.StatusCode
}

// RateLimitError represents an HTTP 429 response.
type RateLimitError struct {
	RetryAfter time.Duration
}

// Error returns the rate limit error message.
func (e *RateLimitError) Error() string {
	return "HTTP 429: rate limited"
}

// HTTPStatusCode returns the upstream HTTP status code.
func (*RateLimitError) HTTPStatusCode() int {
	return http.StatusTooManyRequests
}

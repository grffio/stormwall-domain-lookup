package domains

import "errors"

var (
	// ErrInvalidAPIKey indicates an invalid Stormwall API key.
	ErrInvalidAPIKey = errors.New("invalid API key")
	// ErrInvalidName indicates an invalid domain name.
	ErrInvalidName = errors.New("invalid domain name")
	// ErrInvalidServiceID indicates an invalid service identifier.
	ErrInvalidServiceID = errors.New("invalid service ID")
	// ErrNotFound indicates that the requested domain does not exist.
	ErrNotFound = errors.New("domain not found")
)

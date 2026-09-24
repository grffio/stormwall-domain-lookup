package health

import (
	"fmt"
)

// DependencyError describes a failed dependency health check.
type DependencyError struct {
	Name string
	Err  error
}

// Error returns a human-readable dependency check error.
func (e DependencyError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("dependency %q unhealthy", e.Name)
	}

	return fmt.Sprintf("dependency %q: %v", e.Name, e.Err)
}

// Unwrap returns the underlying dependency error.
func (e DependencyError) Unwrap() error {
	return e.Err
}

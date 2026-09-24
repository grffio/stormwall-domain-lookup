package domains

import (
	"fmt"
	"strings"
)

func validateServiceID(serviceID string) error {
	if strings.TrimSpace(serviceID) == "" {
		return fmt.Errorf("%w: must not be blank", ErrInvalidServiceID)
	}

	if strings.TrimSpace(serviceID) != serviceID {
		return fmt.Errorf("%w: must not contain surrounding whitespace", ErrInvalidServiceID)
	}

	return nil
}

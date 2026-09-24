package domains

import (
	"fmt"
	"strings"
)

func normalizeName(name string) (string, error) {
	name = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(name), "."))

	if name == "" {
		return "", fmt.Errorf("%w: empty domain name", ErrInvalidName)
	}

	if len(name) > 253 {
		return "", fmt.Errorf("%w: domain name exceeds 253 bytes", ErrInvalidName)
	}

	for label := range strings.SplitSeq(name, ".") {
		if !validDomainLabel(label) {
			return "", fmt.Errorf("%w: invalid label %q", ErrInvalidName, label)
		}
	}

	return name, nil
}

func validDomainLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 {
		return false
	}

	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}

	for i := 0; i < len(label); i++ {
		c := label[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}

	return true
}

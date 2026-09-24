package domains

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

func validateAPIKey(apiKey string) error {
	if apiKey == "" {
		return fmt.Errorf("%w: must not be empty", ErrInvalidAPIKey)
	}

	if strings.IndexFunc(apiKey, unicode.IsSpace) >= 0 {
		return fmt.Errorf("%w: must not contain whitespace", ErrInvalidAPIKey)
	}

	return nil
}

func credentialScope(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))

	return hex.EncodeToString(sum[:])
}

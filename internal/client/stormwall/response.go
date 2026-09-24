package stormwall

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/grffio/stormwall-domain-lookup/internal/services/domains"
)

type domainsResponse struct {
	Status  string          `json:"status"`
	Payload *domainsPayload `json:"payload"`
}

type domainsPayload struct {
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	Results []domainResponse `json:"results"`
}

type domainResponse struct {
	ID     int64  `json:"id"`
	Domain string `json:"domain"`
}

func decodeDomains(body []byte) ([]domains.Domain, error) {
	var response domainsResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("invalid response: decode JSON: %w", err)
	}

	if response.Status != "ok" {
		return nil, fmt.Errorf("invalid response: unexpected status %q", response.Status)
	}

	if response.Payload == nil {
		return nil, errors.New("invalid response: missing payload")
	}

	if response.Payload.Results == nil {
		return nil, errors.New("invalid response: missing results array")
	}

	result := make([]domains.Domain, 0, len(response.Payload.Results))

	for i, item := range response.Payload.Results {
		if item.ID <= 0 || strings.TrimSpace(item.Domain) == "" {
			return nil, fmt.Errorf("invalid response: invalid domain at index %d", i)
		}

		result = append(result, domains.Domain{
			ID:   item.ID,
			Name: item.Domain,
		})
	}

	return result, nil
}

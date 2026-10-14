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

type domainsPage struct {
	Total   int
	Limit   int
	Offset  int
	Results []domains.Domain
}

type domainResponse struct {
	ID     int64  `json:"id"`
	Domain string `json:"domain"`
}

func decodeDomains(body []byte) (domainsPage, error) {
	var response domainsResponse

	if err := json.Unmarshal(body, &response); err != nil {
		return domainsPage{}, fmt.Errorf("invalid response: decode JSON: %w", err)
	}

	if response.Status != "ok" {
		return domainsPage{}, fmt.Errorf("invalid response: unexpected status %q", response.Status)
	}

	if response.Payload == nil {
		return domainsPage{}, errors.New("invalid response: missing payload")
	}

	payload := response.Payload

	if payload.Results == nil {
		return domainsPage{}, errors.New("invalid response: missing results array")
	}

	switch {
	case payload.Total < 0:
		return domainsPage{}, errors.New("invalid response: negative total")
	case payload.Limit <= 0:
		return domainsPage{}, errors.New("invalid response: non-positive limit")
	case payload.Offset < 0:
		return domainsPage{}, errors.New("invalid response: negative offset")
	case payload.Offset > payload.Total:
		return domainsPage{}, errors.New("invalid response: offset exceeds total")
	case len(payload.Results) > payload.Limit:
		return domainsPage{}, errors.New("invalid response: results exceed limit")
	case len(payload.Results) > payload.Total-payload.Offset:
		return domainsPage{}, errors.New("invalid response: results exceed total")
	case len(payload.Results) == 0 && payload.Offset < payload.Total:
		return domainsPage{}, errors.New("invalid response: empty page before total")
	}

	result := make([]domains.Domain, 0, len(payload.Results))

	for i, item := range payload.Results {
		if item.ID <= 0 || strings.TrimSpace(item.Domain) == "" {
			return domainsPage{}, fmt.Errorf("invalid response: invalid domain at index %d", i)
		}

		result = append(result, domains.Domain{
			ID:   item.ID,
			Name: item.Domain,
		})
	}

	return domainsPage{
		Total:   payload.Total,
		Limit:   payload.Limit,
		Offset:  payload.Offset,
		Results: result,
	}, nil
}

package domains

// domainResponse represents a domain in an HTTP response.
type domainResponse struct {
	ID     int64  `json:"id"`
	Domain string `json:"domain"`
}

// searchPayload represents domain search results.
type searchPayload struct {
	Total   int              `json:"total"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	Results []domainResponse `json:"results"`
}

// searchResponse represents a Stormwall-compatible search response.
type searchResponse struct {
	Status  string        `json:"status"`
	Payload searchPayload `json:"payload"`
}

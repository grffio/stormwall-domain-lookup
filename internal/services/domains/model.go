package domains

// Domain represents a domain managed by Stormwall.
type Domain struct {
	ID   int64
	Name string
}

// CacheKey uniquely identifies a cached domain within a credential scope.
type CacheKey struct {
	Scope     string
	ServiceID string
	Name      string
}

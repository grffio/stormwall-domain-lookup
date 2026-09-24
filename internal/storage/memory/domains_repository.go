package memory

import (
	"fmt"

	"github.com/grffio/stormwall-domain-lookup/internal/services/domains"

	"github.com/patrickmn/go-cache"
)

const domainKeyPrefix = "domain:"

// DomainsRepository provides cached access to domain records.
type DomainsRepository struct {
	store *Store
}

// NewDomainsRepository creates a repository backed by the given store.
func NewDomainsRepository(store *Store) *DomainsRepository {
	return &DomainsRepository{
		store: store,
	}
}

// Upsert inserts or replaces a domain and resets its TTL.
func (r *DomainsRepository) Upsert(key domains.CacheKey, domain domains.Domain) error {
	if err := validateDomainCacheKey(key); err != nil {
		return err
	}

	if domain.Name == "" {
		return fmt.Errorf("domain name must not be empty")
	}

	if domain.Name != key.Name {
		return fmt.Errorf("domain name %q does not match cache key name %q", domain.Name, key.Name)
	}

	r.store.cache.Set(domainKey(key), domain, cache.DefaultExpiration)

	return nil
}

// Get retrieves an unexpired domain by its exact name.
// A cache miss is reported through the boolean return value.
func (r *DomainsRepository) Get(key domains.CacheKey) (domains.Domain, bool, error) {
	if err := validateDomainCacheKey(key); err != nil {
		return domains.Domain{}, false, err
	}

	value, found := r.store.cache.Get(domainKey(key))
	if !found {
		return domains.Domain{}, false, nil
	}

	domain, ok := value.(domains.Domain)
	if !ok {
		return domains.Domain{}, false, fmt.Errorf("cached domain %q has type %T, want domains.Domain", key.Name, value)
	}

	return domain, true, nil
}

func validateDomainCacheKey(key domains.CacheKey) error {
	if key.Scope == "" {
		return fmt.Errorf("cache scope must not be empty")
	}

	if key.ServiceID == "" {
		return fmt.Errorf("cache service ID must not be empty")
	}

	if key.Name == "" {
		return fmt.Errorf("cache domain name must not be empty")
	}

	return nil
}

func domainKey(key domains.CacheKey) string {
	return fmt.Sprintf(
		"%s%d:%s:%d:%s:%s",
		domainKeyPrefix,
		len(key.Scope),
		key.Scope,
		len(key.ServiceID),
		key.ServiceID,
		key.Name,
	)
}

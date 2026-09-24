package domains

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/grffio/stormwall-domain-lookup/pkg/logx"

	"golang.org/x/sync/singleflight"
)

const (
	cachePhaseInitial             = "initial"
	cachePhaseSingleflightRecheck = "singleflight_recheck"
	lookupSourceCache             = "cache"
	lookupSourceCacheAfterMiss    = "cache_after_miss"
	lookupSourceUpstream          = "upstream"
)

// domainsRepository stores and retrieves cached domains.
type domainsRepository interface {
	Get(key CacheKey) (Domain, bool, error)
	Upsert(key CacheKey, domain Domain) error
}

// domainsClient retrieves domains from Stormwall.
type domainsClient interface {
	Search(ctx context.Context, apiKey, serviceID, name string) ([]Domain, error)
}

// ServiceDeps contains dependencies required by Service.
type ServiceDeps struct {
	Repository    domainsRepository
	Client        domainsClient
	LookupTimeout time.Duration
}

// Service implements domain use cases.
type Service struct {
	repository    domainsRepository
	client        domainsClient
	flights       singleflight.Group
	lookupTimeout time.Duration

	lifecycleMu sync.Mutex
	closing     bool
	shutdown    chan struct{}
	inFlight    sync.WaitGroup

	logger *slog.Logger
}

type lookupResult struct {
	domain Domain
	source string
}

// NewService creates a domain service.
func NewService(deps ServiceDeps, logger *slog.Logger) (*Service, error) {
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	if deps.Repository == nil {
		return nil, errors.New("repository must not be nil")
	}

	if deps.Client == nil {
		return nil, errors.New("client must not be nil")
	}

	if deps.LookupTimeout <= 0 {
		return nil, errors.New("lookup timeout must be positive")
	}

	return &Service{
		repository:    deps.Repository,
		client:        deps.Client,
		lookupTimeout: deps.LookupTimeout,
		shutdown:      make(chan struct{}),
		logger:        logger,
	}, nil
}

// FindExact retrieves a domain from the cache or Stormwall.
func (s *Service) FindExact(ctx context.Context, apiKey, serviceID, name string) (Domain, error) {
	const op = "services.domains.find_exact"
	started := time.Now()

	if err := validateAPIKey(apiKey); err != nil {
		return Domain{}, err
	}

	if err := validateServiceID(serviceID); err != nil {
		return Domain{}, err
	}

	normalizedName, err := normalizeName(name)
	if err != nil {
		return Domain{}, err
	}

	key := CacheKey{
		Scope:     credentialScope(apiKey),
		ServiceID: serviceID,
		Name:      normalizedName,
	}

	log := logx.LoggerFromContext(ctx, s.logger).With(
		logx.String("op", op),
		logx.String("service_id", serviceID),
		logx.String("domain", normalizedName),
	)
	log.DebugContext(ctx, "domain lookup started")

	if cached, found := s.getCached(ctx, key, cachePhaseInitial); found {
		log.DebugContext(
			ctx,
			"domain lookup completed",
			logx.String("source", lookupSourceCache),
			logx.Int("duration_ms", time.Since(started).Milliseconds()),
		)

		return cached, nil
	}

	flightKey := lookupKey(key)

	resultCh := s.flights.DoChan(flightKey, func() (any, error) {
		if !s.beginLookup() {
			return lookupResult{}, context.Canceled
		}
		defer s.inFlight.Done()

		sharedCtx, cancel := s.newLookupContext(ctx)
		defer cancel()

		return s.findExactUncached(sharedCtx, apiKey, key)
	})

	select {
	case <-ctx.Done():
		return Domain{}, ctx.Err()

	case result := <-resultCh:
		if result.Err != nil {
			if errors.Is(result.Err, ErrNotFound) {
				log.DebugContext(
					ctx,
					"domain lookup completed",
					logx.String("result", "not_found"),
					slog.Bool("shared", result.Shared),
					logx.Int("duration_ms", time.Since(started).Milliseconds()),
				)
			} else {
				log.DebugContext(
					ctx,
					"domain lookup failed",
					slog.Bool("shared", result.Shared),
					logx.Int("duration_ms", time.Since(started).Milliseconds()),
					logx.Error(result.Err),
				)
			}

			return Domain{}, result.Err
		}

		lookup, ok := result.Val.(lookupResult)
		if !ok {
			return Domain{}, fmt.Errorf(
				"unexpected singleflight result type %T",
				result.Val,
			)
		}

		log.DebugContext(
			ctx,
			"domain lookup completed",
			logx.String("source", lookup.source),
			slog.Bool("shared", result.Shared),
			logx.Int("duration_ms", time.Since(started).Milliseconds()),
		)

		return lookup.domain, nil
	}
}

func lookupKey(key CacheKey) string {
	return fmt.Sprintf(
		"%d:%s:%d:%s:%s",
		len(key.Scope),
		key.Scope,
		len(key.ServiceID),
		key.ServiceID,
		key.Name,
	)
}

func (s *Service) beginLookup() bool {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()

	if s.closing {
		return false
	}

	s.inFlight.Add(1)

	return true
}

func (s *Service) newLookupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	lookupCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		s.lookupTimeout,
	)

	go func() {
		select {
		case <-s.shutdown:
			cancel()
		case <-lookupCtx.Done():
		}
	}()

	return lookupCtx, cancel
}

func (s *Service) getCached(ctx context.Context, key CacheKey, phase string) (Domain, bool) {
	const op = "cache.domains.get"
	started := time.Now()

	log := logx.LoggerFromContext(ctx, s.logger).With(
		logx.String("op", op),
		logx.String("service_id", key.ServiceID),
		logx.String("domain", key.Name),
		logx.String("phase", phase),
	)

	cached, found, err := s.repository.Get(key)
	if err != nil {
		log.WarnContext(
			ctx,
			"cache lookup failed",
			logx.Int("duration_ms", time.Since(started).Milliseconds()),
			logx.Error(err),
		)

		return Domain{}, false
	}

	result := "miss"
	if found {
		result = "hit"
	}

	log.DebugContext(
		ctx,
		"cache lookup completed",
		logx.String("result", result),
		logx.Int("duration_ms", time.Since(started).Milliseconds()),
	)

	return cached, found
}

func (s *Service) putCached(ctx context.Context, key CacheKey, domain Domain) {
	const op = "cache.domains.put"

	started := time.Now()

	log := logx.LoggerFromContext(ctx, s.logger).With(
		logx.String("op", op),
		logx.String("service_id", key.ServiceID),
		logx.String("domain", key.Name),
	)

	if err := s.repository.Upsert(key, domain); err != nil {
		log.WarnContext(
			ctx,
			"cache write failed",
			logx.Int("duration_ms", time.Since(started).Milliseconds()),
			logx.Error(err),
		)

		return
	}

	log.DebugContext(
		ctx,
		"cache entry stored",
		logx.Int("duration_ms", time.Since(started).Milliseconds()),
	)
}

func (s *Service) findExactUncached(ctx context.Context, apiKey string, key CacheKey) (lookupResult, error) {
	const op = "services.domains.find_exact_uncached"

	log := logx.LoggerFromContext(ctx, s.logger).With(
		logx.String("op", op),
		logx.String("service_id", key.ServiceID),
		logx.String("domain", key.Name),
	)

	// Another request may have populated the cache between the initial miss
	// and this singleflight execution.
	if cached, found := s.getCached(ctx, key, cachePhaseSingleflightRecheck); found {
		return lookupResult{
			domain: cached,
			source: lookupSourceCacheAfterMiss,
		}, nil
	}

	log.DebugContext(ctx, "domain lookup requires upstream request")

	candidates, err := s.client.Search(ctx, apiKey, key.ServiceID, key.Name)
	if err != nil {
		return lookupResult{}, err
	}

	// The shared lookup context may have expired while the upstream response
	// was being completed. Do not mutate the cache after cancellation.
	if err := ctx.Err(); err != nil {
		return lookupResult{}, err
	}

	for _, candidate := range candidates {
		candidateName, err := normalizeName(candidate.Name)
		if err != nil || candidateName != key.Name {
			continue
		}

		candidate.Name = key.Name

		s.putCached(ctx, key, candidate)

		return lookupResult{
			domain: candidate,
			source: lookupSourceUpstream,
		}, nil
	}

	return lookupResult{}, ErrNotFound
}

// Close cancels shared lookups and waits for them to stop.
func (s *Service) Close() error {
	s.lifecycleMu.Lock()

	if !s.closing {
		s.closing = true
		close(s.shutdown)
	}

	s.lifecycleMu.Unlock()

	s.inFlight.Wait()

	return nil
}

package stormwall

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/grffio/stormwall-domain-lookup/internal/services/domains"
	"github.com/grffio/stormwall-domain-lookup/pkg/logx"
)

const domainsPath = "domains"

// Client retrieves domains from the Stormwall API.
// A Client is safe for concurrent use.
type Client struct {
	endpoint url.URL
	http     *http.Client
	cfg      config

	mu            sync.Mutex
	cooldownUntil time.Time

	logger *slog.Logger
}

// NewClient creates a Stormwall API client.
func NewClient(baseURL string, logger *slog.Logger, opts ...Option) (*Client, error) {
	if logger == nil {
		return nil, errors.New("logger must not be nil")
	}

	endpoint, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}

	cfg := defaultConfig()

	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("client option must not be nil")
		}
		if err := opt(&cfg); err != nil {
			return nil, fmt.Errorf("configure client: %w", err)
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("configure client: %w", err)
	}

	return &Client{
		endpoint: *endpoint.JoinPath(domainsPath),
		http: &http.Client{
			Transport: cfg.transport,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		cfg:    cfg,
		logger: logger,
	}, nil
}

// Check verifies direct TLS connectivity to the API endpoint.
// It does not verify authentication or use the configured HTTP proxy.
func (c *Client) Check(ctx context.Context) error {
	host := c.endpoint.Hostname()
	port := c.endpoint.Port()

	if port == "" {
		port = "443"
	}

	dialer := &tls.Dialer{
		Config: &tls.Config{
			ServerName: host,
			MinVersion: tls.VersionTLS12,
		},
	}

	conn, err := dialer.DialContext(
		ctx,
		"tcp",
		net.JoinHostPort(host, port),
	)
	if err != nil {
		return fmt.Errorf("check tls: %w", err)
	}

	_ = conn.Close()

	return nil
}

// Search retrieves domains matching the filter.
func (c *Client) Search(ctx context.Context, apiKey, serviceID, filter string) ([]domains.Domain, error) {
	const op = "client.stormwall.search"
	started := time.Now()

	log := logx.LoggerFromContext(ctx, c.logger).With(
		logx.String("op", op),
		logx.String("service_id", serviceID),
		logx.String("filter", filter),
	)

	log.DebugContext(ctx, "upstream domain search started")

	body, err := c.get(ctx, apiKey, c.searchURL(serviceID, filter), log)
	if err != nil {
		log.DebugContext(
			ctx,
			"upstream domain search failed",
			logx.Int("duration_ms", time.Since(started).Milliseconds()),
			logx.Error(err),
		)
		return nil, fmt.Errorf("search domains: %w", err)
	}

	result, err := decodeDomains(body)
	if err != nil {
		log.DebugContext(
			ctx,
			"upstream response decoding failed",
			logx.Int("response_bytes", len(body)),
			logx.Int("duration_ms", time.Since(started).Milliseconds()),
			logx.Error(err),
		)
		return nil, fmt.Errorf("search domains: %w", err)
	}

	return result, nil
}

func (c *Client) searchURL(serviceID, filter string) string {
	endpoint := c.endpoint

	query := endpoint.Query()

	if serviceID != "" {
		query.Set("serviceId", serviceID)
	}
	if filter != "" {
		query.Set("search", filter)
	}

	endpoint.RawQuery = query.Encode()

	return endpoint.String()
}

type attemptResult struct {
	body          []byte
	err           error
	statusCode    int
	retryAfter    time.Duration
	hasRetryAfter bool
}

type attemptTiming struct {
	permitWait time.Duration
	request    time.Duration
}

func (c *Client) get(ctx context.Context, apiKey, target string, log *slog.Logger) ([]byte, error) {
	started := time.Now()
	maxAttempts := c.cfg.retries + 1

	for attempt := range maxAttempts {
		result, timing, err := c.executeAttempt(ctx, apiKey, target)
		if err != nil {
			log.DebugContext(
				ctx,
				"upstream request attempt aborted",
				logx.Int("attempt", attempt+1),
				logx.Int("max_attempts", maxAttempts),
				logx.Int("permit_wait_ms", timing.permitWait.Milliseconds()),
				logx.Error(err),
			)

			return nil, err
		}

		logAttempt(ctx, log, attempt, maxAttempts, timing, result)

		if result.err == nil {
			logRecovery(ctx, log, attempt, maxAttempts, started)

			return result.body, nil
		}

		// Prefer caller cancellation over retrying a completed failed attempt.
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		if !retryable(result.err) {
			return nil, result.err
		}

		plan, planErr := c.retryPlan(attempt, result)

		if plan.sharedCooldown {
			c.setCooldown(plan.delay)
		}

		if planErr != nil {
			return nil, planErr
		}

		if attempt == maxAttempts-1 {
			return nil, fmt.Errorf("request failed after %d attempts: %w", attempt+1, result.err)
		}

		logRetry(ctx, log, attempt, maxAttempts, timing, result, plan)

		if err := waitForRetry(ctx, plan); err != nil {
			return nil, err
		}
	}

	return nil, errors.New("retry attempts exhausted")
}

func logAttempt(ctx context.Context, log *slog.Logger, attempt, maxAttempts int, timing attemptTiming, result attemptResult) {
	attrs := []slog.Attr{
		logx.Int("attempt", attempt+1),
		logx.Int("max_attempts", maxAttempts),
		logx.Int("permit_wait_ms", timing.permitWait.Milliseconds()),
		logx.Int("request_duration_ms", timing.request.Milliseconds()),
	}

	if result.statusCode > 0 {
		attrs = append(attrs, logx.Int("upstream_status", result.statusCode))
	}

	if len(result.body) > 0 {
		attrs = append(attrs, logx.Int("response_bytes", len(result.body)))
	}

	if result.err != nil {
		attrs = append(attrs, logx.Error(result.err))
	}

	log.LogAttrs(ctx, slog.LevelDebug, "upstream request attempt completed", attrs...)
}

func logRecovery(ctx context.Context, log *slog.Logger, attempt int, maxAttempts int, started time.Time) {
	if attempt == 0 {
		return
	}

	log.InfoContext(
		ctx,
		"upstream request recovered after retry",
		logx.Int("attempts", attempt+1),
		logx.Int("max_attempts", maxAttempts),
		logx.Duration("duration", time.Since(started)),
	)
}

func waitForRetry(ctx context.Context, plan retryPlan) error {
	if plan.sharedCooldown {
		return nil
	}

	if err := sleep(ctx, plan.delay); err != nil {
		return fmt.Errorf("wait before retry: %w", err)
	}

	return nil
}

func (c *Client) executeAttempt(ctx context.Context, apiKey, target string) (attemptResult, attemptTiming, error) {
	var timing attemptTiming
	permitStarted := time.Now()

	if err := c.waitForPermit(ctx); err != nil {
		timing.permitWait = time.Since(permitStarted)
		return attemptResult{}, timing, fmt.Errorf("acquire request permit: %w", err)
	}

	timing.permitWait = time.Since(permitStarted)

	attemptCtx, cancel := context.WithTimeout(ctx, c.cfg.attemptTimeout)
	defer cancel()

	requestStarted := time.Now()
	result := c.doOnce(attemptCtx, apiKey, target)
	timing.request = time.Since(requestStarted)

	return result, timing, nil
}

type retryPlan struct {
	delay          time.Duration
	sharedCooldown bool
}

func (c *Client) retryPlan(attempt int, result attemptResult) (retryPlan, error) {
	sharedCooldown := result.statusCode == http.StatusTooManyRequests ||
		result.statusCode == http.StatusServiceUnavailable &&
			result.hasRetryAfter

	if result.hasRetryAfter &&
		result.retryAfter > c.cfg.maxRetryAfter {
		return retryPlan{
				delay:          result.retryAfter,
				sharedCooldown: sharedCooldown,
			},
			fmt.Errorf("retry-after %s exceeds maximum %s: %w", result.retryAfter, c.cfg.maxRetryAfter, result.err)
	}

	delay := c.backoff(attempt)

	if result.hasRetryAfter {
		delay = max(delay, result.retryAfter)
	}

	return retryPlan{
		delay:          delay,
		sharedCooldown: sharedCooldown,
	}, nil
}

func (c *Client) doOnce(ctx context.Context, apiKey, target string) attemptResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return attemptResult{
			err: fmt.Errorf("create request: %w", err),
		}
	}

	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return attemptResult{
			err: fmt.Errorf("send request: %w", err),
		}
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		body, err := c.readBody(resp.Body)

		return attemptResult{
			body:       body,
			err:        err,
			statusCode: resp.StatusCode,
		}
	}

	endpoint := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Host, req.URL.EscapedPath())

	result := attemptResult{
		statusCode: resp.StatusCode,
	}

	if resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode == http.StatusServiceUnavailable {
		result.retryAfter, result.hasRetryAfter = parseRetryAfter(
			resp.Header.Get("Retry-After"),
			time.Now(),
		)
	}

	// Bound the amount of data consumed on error responses.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 8<<10))

	if resp.StatusCode == http.StatusTooManyRequests {
		result.err = fmt.Errorf("%s %s: %w", req.Method, endpoint, &RateLimitError{
			RetryAfter: result.retryAfter,
		})

		return result
	}

	result.err = &StatusError{
		StatusCode: resp.StatusCode,
		Method:     req.Method,
		Endpoint:   endpoint,
	}

	return result
}

func (c *Client) readBody(body io.Reader) ([]byte, error) {
	limit := c.cfg.maxResponseBytes

	data, err := io.ReadAll(
		io.LimitReader(body, limit+1),
	)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if int64(len(data)) > limit {
		return nil, fmt.Errorf("invalid response: body exceeds %d bytes", limit)
	}

	return data, nil
}

func logRetry(ctx context.Context, log *slog.Logger, attempt, maxAttempts int, timing attemptTiming, result attemptResult, plan retryPlan) {
	attrs := []slog.Attr{
		logx.Int("attempt", attempt+1),
		logx.Int("next_attempt", attempt+2),
		logx.Int("max_attempts", maxAttempts),
		logx.Int("permit_wait_ms", timing.permitWait.Milliseconds()),
		logx.Int("request_duration_ms", timing.request.Milliseconds()),
		logx.Int("retry_delay_ms", plan.delay.Milliseconds()),
		logx.Error(result.err),
	}

	if result.statusCode > 0 {
		attrs = append(attrs, logx.Int("upstream_status", result.statusCode))
	}

	if result.hasRetryAfter {
		attrs = append(attrs, logx.Duration("retry_after_ms", time.Duration(result.retryAfter.Milliseconds())))
	}

	if plan.sharedCooldown {
		log.LogAttrs(ctx, slog.LevelWarn, "upstream shared cooldown scheduled", attrs...)
		return
	}

	log.LogAttrs(ctx, slog.LevelDebug, "upstream request retry scheduled", attrs...)
}

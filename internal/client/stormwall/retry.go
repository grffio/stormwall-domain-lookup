package stormwall

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func retryable(err error) bool {
	if _, ok := errors.AsType[*RateLimitError](err); ok {
		return true
	}

	if statusErr, ok := errors.AsType[*StatusError](err); ok {
		return retryableStatus(statusErr.StatusCode)
	}

	return retryableNetworkError(err)
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true

	default:
		return false
	}
}

func retryableNetworkError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}

	if errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) {
		return true
	}

	if netErr, ok := errors.AsType[net.Error](err); ok {
		return netErr.Timeout()
	}

	return false
}

func (c *Client) backoff(attempt int) time.Duration {
	delay := c.cfg.baseBackoff

	for range attempt {
		if delay >= c.cfg.maxBackoff/2 {
			delay = c.cfg.maxBackoff
			break
		}

		delay *= 2
	}

	return jitter(delay)
}

func jitter(delay time.Duration) time.Duration {
	half := delay / 2
	spread := delay - half

	return half + time.Duration(
		rand.Int63n(int64(spread)+1), //nolint:gosec // Backoff jitter does not require cryptographic randomness.
	)
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	seconds, err := strconv.ParseInt(value, 10, 64)
	if err == nil &&
		seconds >= 0 &&
		seconds <= math.MaxInt64/int64(time.Second) {
		return time.Duration(seconds) * time.Second, true
	}

	retryAt, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}

	return max(0, retryAt.Sub(now)), true
}

func sleep(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) setCooldown(delay time.Duration) {
	if delay <= 0 {
		return
	}

	until := time.Now().Add(delay)

	c.mu.Lock()

	if until.After(c.cooldownUntil) {
		c.cooldownUntil = until
	}

	c.mu.Unlock()
}

func (c *Client) waitCooldown(ctx context.Context) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		c.mu.Lock()
		remaining := time.Until(c.cooldownUntil)
		c.mu.Unlock()

		if remaining <= 0 {
			return nil
		}

		if err := sleep(ctx, remaining); err != nil {
			return err
		}
	}
}

func (c *Client) waitForPermit(ctx context.Context) error {
	if err := c.waitCooldown(ctx); err != nil {
		return fmt.Errorf("wait for cooldown: %w", err)
	}

	if c.cfg.limiter == nil {
		return nil
	}

	if err := c.cfg.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("wait for rate limiter: %w", err)
	}

	if err := c.waitCooldown(ctx); err != nil {
		return fmt.Errorf("wait for cooldown: %w", err)
	}

	return nil
}

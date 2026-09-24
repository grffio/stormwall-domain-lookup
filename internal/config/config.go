package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Config holds the application's runtime configuration.
type Config struct {
	Env                  string        `default:"production" enum:"local,develop,production" help:"Runtime environment."`
	ListenAddr           string        `default:":8080" help:"HTTP listen address (host:port)."`
	CacheTTL             time.Duration `default:"3m" help:"Time to cache domain records."`
	CacheCleanupInterval time.Duration `default:"1m" help:"Interval for removing expired cache records (0 disables cleanup)."`

	Auth      AuthConfig      `embed:"" prefix:"auth-"`
	Stormwall StormwallConfig `embed:"" prefix:"stormwall-"`
}

// Validate checks configuration invariants independently of the CLI parser.
func (c Config) Validate() error {
	if err := validateListenAddr(c.ListenAddr); err != nil {
		return fmt.Errorf("listen address: %w", err)
	}

	if c.CacheTTL <= 0 {
		return errors.New("cache ttl must be greater than zero")
	}

	if c.CacheCleanupInterval < 0 {
		return errors.New("cache cleanup interval must not be negative")
	}

	if err := c.Auth.Validate(); err != nil {
		return fmt.Errorf("auth configuration: %w", err)
	}

	if err := c.Stormwall.Validate(); err != nil {
		return fmt.Errorf("stormwall configuration: %w", err)
	}

	return nil
}

func validateListenAddr(addr string) error {
	if strings.TrimSpace(addr) == "" {
		return errors.New("must not be empty")
	}

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("split host and port: %w", err)
	}

	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("listen port must be between 1 and 65535")
	}

	return nil
}

// AuthConfig holds credentials accepted by the wrapper API.
type AuthConfig struct {
	Token string `help:"Optional Bearer token accepted by this API. Authentication is disabled when empty."`
}

// Validate checks the incoming authentication token configuration.
func (a AuthConfig) Validate() error {
	if strings.IndexFunc(a.Token, unicode.IsSpace) >= 0 {
		return errors.New("token must not contain whitespace")
	}

	return nil
}

// StormwallConfig holds the upstream API endpoint and credentials.
type StormwallConfig struct {
	BaseURL string `default:"https://api.stormwall.pro/v3/" help:"Stormwall API base URL."`
}

// Validate checks the Stormwall API connection settings.
func (c StormwallConfig) Validate() error {
	if err := validateBaseURL(c.BaseURL); err != nil {
		return fmt.Errorf("base url: %w", err)
	}

	return nil
}

func validateBaseURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("must not be empty")
	}

	if strings.TrimSpace(raw) != raw {
		return errors.New("must not contain surrounding whitespace")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	if u.Scheme != "https" || u.Hostname() == "" || u.Opaque != "" {
		return errors.New("must be an absolute https url")
	}

	if u.User != nil {
		return errors.New("must not contain credentials")
	}

	if u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return errors.New("must not contain query or fragment")
	}

	return nil
}

package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

// Load parses config from flags/env and validates it.
func Load(appName, appDescription, configPrefix string) (Config, error) {
	var cfg Config

	// Configure kong parser with application details and environment variable support.
	parser, err := kong.New(&cfg,
		kong.Name(appName),
		kong.Description(appDescription),
		kong.DefaultEnvars(normalizePrefix(configPrefix)),
		kong.UsageOnError(),
	)
	if err != nil {
		return Config{}, fmt.Errorf("create config parser: %w", err)
	}

	// Parse command-line arguments and environment variables.
	args := normalizeShortEquals(os.Args[1:])
	_, err = parser.Parse(args)
	if err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}

	return cfg, nil
}

// normalizePrefix converts string (e.g. "my-app") into an environment variable prefix (e.g. "MY_APP").
func normalizePrefix(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
}

// normalizeShortEquals rewrites short flags passed as "-x=value" into "-x", "value".
func normalizeShortEquals(args []string) []string {
	out := make([]string, 0, len(args))

	for _, arg := range args {
		// Accept only: "-x=value" (single-letter short flag).
		if len(arg) >= 4 && arg[0] == '-' && arg[1] != '-' && arg[2] == '=' {
			out = append(out, arg[:2], arg[3:]) // "-x=value" -> "-x", "value".
			continue
		}
		out = append(out, arg)
	}

	return out
}

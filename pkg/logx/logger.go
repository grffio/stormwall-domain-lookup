package logx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/phsym/console-slog"
)

// Mode identifies a logger preset.
type Mode string

const (
	// ModeLocal enables human-readable debug logs with source locations.
	ModeLocal Mode = "local"
	// ModeDevelop enables structured debug logs with source locations.
	ModeDevelop Mode = "develop"
	// ModeProduction enables structured info logs without source locations.
	ModeProduction Mode = "production"
)

// Option customizes logger construction.
type Option func(*options)

type options struct {
	output    io.Writer
	level     slog.Leveler
	addSource bool
}

// WithOutput sets the logger output destination.
func WithOutput(output io.Writer) Option {
	return func(opts *options) {
		opts.output = output
	}
}

// WithLevel overrides the default logging level for the selected mode.
func WithLevel(level slog.Leveler) Option {
	return func(opts *options) {
		opts.level = level
	}
}

// WithAddSource overrides whether log records include source locations.
func WithAddSource(enabled bool) Option {
	return func(opts *options) {
		opts.addSource = enabled
	}
}

// ParseMode converts a raw mode string into a supported logging mode.
func ParseMode(mode string) (Mode, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "local":
		return ModeLocal, nil
	case "dev", "devel", "develop", "development":
		return ModeDevelop, nil
	case "prod", "production":
		return ModeProduction, nil
	default:
		return "", fmt.Errorf("unsupported logging mode: %q", mode)
	}
}

// NewLogger creates a logger for the selected mode.
func NewLogger(mode Mode, opts ...Option) (*slog.Logger, error) {
	cfg, err := buildOptions(mode, opts...)
	if err != nil {
		return nil, err
	}

	handler, err := newHandler(mode, cfg)
	if err != nil {
		return nil, err
	}

	return slog.New(handler), nil
}

// NewLoggerFromMode creates a logger from a raw mode string.
func NewLoggerFromMode(mode string, opts ...Option) (*slog.Logger, error) {
	parsedMode, err := ParseMode(mode)
	if err != nil {
		return nil, err
	}

	return NewLogger(parsedMode, opts...)
}

func buildOptions(mode Mode, opts ...Option) (options, error) {
	defaultLevel, defaultAddSource, _, err := defaultsForMode(mode)
	if err != nil {
		return options{}, err
	}

	cfg := options{
		output:    os.Stdout,
		level:     defaultLevel,
		addSource: defaultAddSource,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	if cfg.output == nil {
		cfg.output = io.Discard
	}

	if cfg.level == nil {
		cfg.level = defaultLevel
	}

	return cfg, nil
}

func newHandler(mode Mode, opts options) (slog.Handler, error) {
	_, _, useConsole, err := defaultsForMode(mode)
	if err != nil {
		return nil, err
	}

	if useConsole {
		return console.NewHandler(opts.output, &console.HandlerOptions{
			Level:     opts.level,
			AddSource: opts.addSource,
		}), nil
	}

	return slog.NewJSONHandler(opts.output, &slog.HandlerOptions{
		Level:     opts.level,
		AddSource: opts.addSource,
	}), nil
}

func defaultsForMode(mode Mode) (slog.Leveler, bool, bool, error) {
	switch mode {
	case ModeLocal:
		return slog.LevelDebug, true, true, nil
	case ModeDevelop:
		return slog.LevelDebug, true, false, nil
	case ModeProduction:
		return slog.LevelInfo, false, false, nil
	default:
		return nil, false, false, fmt.Errorf("unsupported logging mode: %q", mode)
	}
}

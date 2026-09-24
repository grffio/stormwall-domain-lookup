package logx

import (
	"log/slog"
	"strings"
	"time"
)

// Any returns a "slog.Attr" for an arbitrary value.
// It returns an empty attribute if the key is empty.
func Any(key string, value any) slog.Attr {
	if key == "" {
		return slog.Attr{}
	}
	return slog.Any(key, value)
}

// IntLike represents integer types supported by the Int attribute helper.
type IntLike interface {
	~int | ~int64
}

// Int returns a "slog.Attr" for an integer value (int or int64).
// It returns an empty attribute if the key is empty.
func Int[T IntLike](key string, value T) slog.Attr {
	if key == "" {
		return slog.Attr{}
	}
	return slog.Int64(key, int64(value))
}

// String returns a "slog.Attr" for a string value.
// It returns an empty attribute if the key is empty.
func String(key, value string) slog.Attr {
	if key == "" {
		return slog.Attr{}
	}
	return slog.String(key, value)
}

// Strings returns a "slog.Attr" for a slice of strings, joined by commas.
// It returns an empty attribute if the key is empty or the slice is nil or empty.
func Strings(key string, values []string) slog.Attr {
	if key == "" || len(values) == 0 {
		return slog.Attr{}
	}
	return slog.String(key, strings.Join(values, ","))
}

// StringSlice returns a structured array attribute (better for JSON logs).
func StringSlice(key string, values []string) slog.Attr {
	if key == "" || len(values) == 0 {
		return slog.Attr{}
	}
	return slog.Any(key, values)
}

// Duration returns a "slog.Attr" for a "time.Duration" value.
// It returns an empty attribute if the key is empty.
func Duration(key string, value time.Duration) slog.Attr {
	if key == "" {
		return slog.Attr{}
	}
	return slog.Duration(key, value)
}

// Time returns a "slog.Attr" for a "time.Time" value.
// It returns an empty attribute if the key is empty.
func Time(key string, value time.Time) slog.Attr {
	if key == "" {
		return slog.Attr{}
	}
	return slog.Time(key, value)
}

// Error returns a structured error attribute.
func Error(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}

	return slog.String("error", err.Error())
}

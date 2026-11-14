package store

import (
	"strings"
	"time"
)

// ParseFlexibleTime parses a timestamp string using multiple possible layouts
func ParseFlexibleTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 -0700",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 +0000 UTC",
		"2006-01-02 15:04:05 -0700 -0700",
		"2006-01-02 15:04:05 +0000 UTC",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}

	// Attempt with split spaces if monotonic clock is appended
	parts := strings.Split(s, " m=")
	if len(parts) > 1 {
		return ParseFlexibleTime(parts[0])
	}

	return time.Time{}
}

// FormatTime formats a time.Time into standard RFC3339 string for SQLite/Postgres
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

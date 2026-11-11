package store

import (
	"fmt"
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

// FlexibleTime scans either a TEXT timestamp or a driver time.Time.
// SQLite stores timestamps as text. PostgreSQL returns time.Time for timestamptz.
type FlexibleTime struct {
	Time time.Time
}

func (f *FlexibleTime) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		f.Time = time.Time{}
		return nil
	case time.Time:
		f.Time = value
		return nil
	case string:
		f.Time = ParseFlexibleTime(value)
		return nil
	case []byte:
		f.Time = ParseFlexibleTime(string(value))
		return nil
	default:
		return fmt.Errorf("cannot scan %T as time", src)
	}
}

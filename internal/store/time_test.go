package store

import (
	"testing"
	"time"
)

func TestFlexibleTimeScansDriverTimeAndText(t *testing.T) {
	instant := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	var parsed FlexibleTime
	if err := parsed.Scan(instant); err != nil || !parsed.Time.Equal(instant) {
		t.Fatalf("time.Time scan = %v, %v", parsed.Time, err)
	}
	if err := parsed.Scan("2026-06-01T08:00:00Z"); err != nil || !parsed.Time.Equal(instant) {
		t.Fatalf("text scan = %v, %v", parsed.Time, err)
	}
	if err := parsed.Scan(nil); err != nil || !parsed.Time.IsZero() {
		t.Fatalf("null scan = %v, %v", parsed.Time, err)
	}
}

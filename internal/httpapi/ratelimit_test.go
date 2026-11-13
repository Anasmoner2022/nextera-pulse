package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestAttemptLimiterWindow(t *testing.T) {
	now := time.Unix(0, 0)
	l := newAttemptLimiter(2, time.Minute)
	l.now = func() time.Time { return now }
	l.add("k")
	if blocked, _ := l.blocked("k"); blocked {
		t.Fatal("blocked after 1")
	}
	l.add("k")
	if blocked, wait := l.blocked("k"); !blocked || wait != time.Minute {
		t.Fatalf("not blocked after 2 (wait %v)", wait)
	}
	now = now.Add(time.Minute)
	if blocked, _ := l.blocked("k"); blocked {
		t.Fatal("still blocked after the window")
	}
	l.add("k")
	l.add("k")
	l.reset("k")
	if blocked, _ := l.blocked("k"); blocked {
		t.Fatal("reset did not clear")
	}
}

func TestClientIPIgnoresForwardedForUnlessTrusted(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/v1/session", nil)
	r.RemoteAddr = "10.0.0.5:4321"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := clientIP(r, false); got != "10.0.0.5" {
		t.Fatalf("untrusted = %s", got)
	}
	if got := clientIP(r, true); got != "203.0.113.9" {
		t.Fatalf("trusted = %s", got)
	}
}

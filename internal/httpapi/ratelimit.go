package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// attemptLimiter is a fixed-window counter per key (an IP or a login).
type attemptLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    func() time.Time
	hits   map[string]*attemptWindow
}

type attemptWindow struct {
	start time.Time
	count int
}

func newAttemptLimiter(limit int, window time.Duration) *attemptLimiter {
	return &attemptLimiter{limit: limit, window: window, now: time.Now, hits: map[string]*attemptWindow{}}
}

// blocked reports whether key is over its limit, and when it may try again.
func (l *attemptLimiter) blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.hits[key]
	now := l.now()
	if w == nil || now.Sub(w.start) >= l.window {
		return false, 0
	}
	if w.count >= l.limit {
		return true, l.window - now.Sub(w.start)
	}
	return false, 0
}

// add counts one attempt for key.
func (l *attemptLimiter) add(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	w := l.hits[key]
	if w == nil || now.Sub(w.start) >= l.window {
		w = &attemptWindow{start: now}
		l.hits[key] = w
	}
	w.count++
	if len(l.hits) > 10000 {
		for k, v := range l.hits {
			if now.Sub(v.start) >= l.window {
				delete(l.hits, k)
			}
		}
	}
}

// reset clears key (after a successful sign-in).
func (l *attemptLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.hits, key)
}

// clientIP is the TCP peer address, or the first X-Forwarded-For entry when the
// API runs behind a trusted reverse proxy (TRUST_PROXY_HEADERS=true).
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first, _, _ := strings.Cut(forwarded, ",")
			if ip := net.ParseIP(strings.TrimSpace(first)); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

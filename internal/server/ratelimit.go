package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateWindow struct {
	start time.Time
	count int
}

// rateLimiter is a fixed-window limiter keyed by client IP.
type rateLimiter struct {
	mu      sync.Mutex
	windows map[string]rateWindow
	limit   int
	window  time.Duration
	now     func() time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		windows: make(map[string]rateWindow),
		limit:   limit,
		window:  window,
		now:     time.Now,
	}
}

func (l *rateLimiter) allow(key string) bool {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.windows[key]
	if !ok || now.Sub(entry.start) >= l.window {
		l.windows[key] = rateWindow{start: now, count: 1}
		l.prune(now)
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.windows[key] = entry
	return true
}

// prune removes expired windows once the map has grown enough to be worth
// scanning.
func (l *rateLimiter) prune(now time.Time) {
	if len(l.windows) <= 1000 {
		return
	}
	for key, entry := range l.windows {
		if now.Sub(entry.start) >= l.window {
			delete(l.windows, key)
		}
	}
}

// clientIP returns the client address used for rate limiting. The application
// listens on loopback behind nginx, so it trusts the proxy headers that nginx
// overwrites: X-Real-IP first, then the last X-Forwarded-For entry.
func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if ip := strings.TrimSpace(parts[len(parts)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

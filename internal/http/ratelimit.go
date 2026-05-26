package apphttp

import (
	"container/list"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type rateLimiterConfig struct {
	Limit    int
	Window   time.Duration
	Capacity int
	Now      func() time.Time
}

type rateEntry struct {
	key       string
	timestamp time.Time
	count     int
}

type rateLimiter struct {
	cfg     rateLimiterConfig
	mu      sync.Mutex
	entries map[string]*list.Element
	lru     *list.List
}

func newRateLimiter(cfg rateLimiterConfig) *rateLimiter {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 50_000
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &rateLimiter{cfg: cfg, entries: make(map[string]*list.Element), lru: list.New()}
}

// allow returns true when key is below the configured limit. It bumps the
// in-window counter and rotates the window when the previous window has fully
// elapsed.
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.cfg.Now()
	if el, ok := l.entries[key]; ok {
		e := el.Value.(*rateEntry)
		if now.Sub(e.timestamp) >= l.cfg.Window {
			e.timestamp = now
			e.count = 0
		}
		if e.count >= l.cfg.Limit {
			l.lru.MoveToFront(el)
			return false
		}
		e.count++
		l.lru.MoveToFront(el)
		return true
	}
	e := &rateEntry{key: key, timestamp: now, count: 1}
	el := l.lru.PushFront(e)
	l.entries[key] = el
	if l.lru.Len() > l.cfg.Capacity {
		oldest := l.lru.Back()
		if oldest != nil {
			delete(l.entries, oldest.Value.(*rateEntry).key)
			l.lru.Remove(oldest)
		}
	}
	return true
}

// retryAfter returns the remaining seconds in the current window for key,
// rounded up to at least 1.
func (l *rateLimiter) retryAfter(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	el, ok := l.entries[key]
	if !ok {
		return int(l.cfg.Window.Seconds())
	}
	e := el.Value.(*rateEntry)
	remaining := l.cfg.Window - l.cfg.Now().Sub(e.timestamp)
	if remaining <= 0 {
		return 1
	}
	secs := int(remaining.Seconds())
	if secs < 1 {
		secs = 1
	}
	return secs
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if real := r.Header.Get("X-Real-IP"); real != "" {
		return strings.TrimSpace(real)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func rateLimitByIP(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			if !rl.allow(ip) {
				writeRateLimited(w, rl.retryAfter(ip))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rateLimitByUser(rl *rateLimiter, auth AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := "ip:" + extractIP(r)
			if hdr := r.Header.Get("Authorization"); strings.HasPrefix(hdr, "Bearer ") {
				if user, err := auth.UserFromAccessToken(r.Context(), strings.TrimPrefix(hdr, "Bearer ")); err == nil {
					key = "u:" + user.ID
				}
			}
			if !rl.allow(key) {
				writeRateLimited(w, rl.retryAfter(key))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimited(w http.ResponseWriter, retryAfter int) {
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": "rate_limited", "message": "too many requests"},
	})
}

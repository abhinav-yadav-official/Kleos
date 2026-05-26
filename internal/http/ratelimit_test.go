package apphttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterAllowsBelowLimit(t *testing.T) {
	rl := newRateLimiter(rateLimiterConfig{
		Limit:    5,
		Window:   time.Second,
		Capacity: 100,
		Now:      time.Now,
	})
	for i := 0; i < 5; i++ {
		if !rl.allow("k1") {
			t.Fatalf("call %d should be allowed", i+1)
		}
	}
	if rl.allow("k1") {
		t.Fatal("6th call should be blocked")
	}
	if !rl.allow("k2") {
		t.Fatal("different key should not be blocked")
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	rl := newRateLimiter(rateLimiterConfig{
		Limit:    2,
		Window:   time.Minute,
		Capacity: 10,
		Now:      func() time.Time { return now },
	})
	if !rl.allow("k") {
		t.Fatal("first allowed")
	}
	if !rl.allow("k") {
		t.Fatal("second allowed")
	}
	if rl.allow("k") {
		t.Fatal("third blocked")
	}
	now = now.Add(61 * time.Second)
	if !rl.allow("k") {
		t.Fatal("after window should allow again")
	}
}

func TestRateLimitMiddlewareReturns429(t *testing.T) {
	rl := newRateLimiter(rateLimiterConfig{Limit: 2, Window: time.Minute, Capacity: 10, Now: time.Now})
	handler := rateLimitByIP(rl)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/x", nil)
		req.RemoteAddr = "1.2.3.4:5555"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: expected 200, got %d", i+1, rec.Code)
		}
	}
	req := httptest.NewRequest("GET", "/x", nil)
	req.RemoteAddr = "1.2.3.4:5555"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestRateLimiterCapacityEviction(t *testing.T) {
	rl := newRateLimiter(rateLimiterConfig{Limit: 1, Window: time.Minute, Capacity: 2, Now: time.Now})
	rl.allow("a")
	rl.allow("b")
	rl.allow("c") // should evict oldest
	rl.mu.Lock()
	n := len(rl.entries)
	rl.mu.Unlock()
	if n > 2 {
		t.Fatalf("capacity not enforced: %d entries", n)
	}
}

func TestExtractIP(t *testing.T) {
	cases := []struct {
		name string
		req  func() *http.Request
		want string
	}{
		{"remoteAddrOnly", func() *http.Request {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "1.2.3.4:1234"
			return r
		}, "1.2.3.4"},
		{"xForwardedFor", func() *http.Request {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "127.0.0.1:1"
			r.Header.Set("X-Forwarded-For", "9.9.9.9, 10.0.0.1")
			return r
		}, "9.9.9.9"},
		{"xRealIP", func() *http.Request {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = "127.0.0.1:1"
			r.Header.Set("X-Real-IP", "8.8.8.8")
			return r
		}, "8.8.8.8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractIP(c.req())
			if got != c.want {
				t.Fatalf("extractIP = %q want %q", got, c.want)
			}
		})
	}
}

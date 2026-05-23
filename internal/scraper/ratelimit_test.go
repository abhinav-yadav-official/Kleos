package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestLimitedClientBlocksUntilTokenAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// 100ms between tokens, burst 1.
	lim := rate.NewLimiter(rate.Every(100*time.Millisecond), 1)
	client := LimitedClient(srv.Client(), lim)

	ctx := context.Background()
	start := time.Now()
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("req %d: %v", i, err)
		}
		_ = resp.Body.Close()
	}
	elapsed := time.Since(start)
	// First request immediate (burst), next two each wait ~100ms.
	if elapsed < 180*time.Millisecond {
		t.Fatalf("elapsed = %s, expected >= 180ms", elapsed)
	}
}

func TestSourceLimits(t *testing.T) {
	defaultLimit, _ := SourceLimits("greenhouse")
	if defaultLimit != rate.Every(2*time.Second) {
		t.Errorf("default limit = %v", defaultLimit)
	}
	linkedinLimit, _ := SourceLimits("linkedin")
	if linkedinLimit != rate.Every(5*time.Second) {
		t.Errorf("linkedin limit = %v", linkedinLimit)
	}
}

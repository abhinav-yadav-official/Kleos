package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFragileSwallowsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "blocked", http.StatusForbidden)
	}))
	defer srv.Close()

	f := newFragile("test", srv.URL, srv.Client())
	got, err := f.Scrape(context.Background(), ScrapeParams{})
	if err != nil {
		t.Fatalf("fragile must not return error on 403, got: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d jobs, want 0", len(got))
	}
}

func TestFragileSwallowsTransportError(t *testing.T) {
	// Closed server — Do() returns transport error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	f := newFragile("test", url, &http.Client{})
	got, err := f.Scrape(context.Background(), ScrapeParams{})
	if err != nil {
		t.Fatalf("fragile must not return transport error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d jobs", len(got))
	}
}

func TestFragileNames(t *testing.T) {
	for name, ctor := range map[string]func(*http.Client) Scraper{
		"wellfound": NewWellfound,
		"indeed":    NewIndeed,
		"naukri":    NewNaukri,
		"linkedin":  NewLinkedIn,
	} {
		if got := ctor(nil).Name(); got != name {
			t.Errorf("ctor for %q returned Name()=%q", name, got)
		}
	}
}

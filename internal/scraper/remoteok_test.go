package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const remoteokSample = `[
  {"legal": "All rights reserved"},
  {
    "id": "1234567",
    "slug": "acme-senior-engineer",
    "company": "Acme",
    "position": "Senior Engineer",
    "tags": ["go", "postgres"],
    "description": "<p>Build cool things in Go.</p>",
    "location": "Worldwide",
    "url": "https://remoteok.com/remote-jobs/1234567",
    "date": "2026-04-10T14:30:00+00:00",
    "epoch": 1776085800
  },
  {
    "id": "7654321",
    "slug": "globex-designer",
    "company": "Globex",
    "position": "Product Designer",
    "tags": ["figma"],
    "description": "<p>Design things.</p>",
    "location": "Anywhere",
    "url": "https://remoteok.com/remote-jobs/7654321",
    "epoch": 1775999400
  }
]`

func TestRemoteOKScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing user agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(remoteokSample))
	}))
	defer srv.Close()

	ro := NewRemoteOK(srv.Client(), srv.URL)
	if ro.Name() != "remoteok" {
		t.Fatalf("name = %q", ro.Name())
	}
	got, err := ro.Scrape(context.Background(), ScrapeParams{})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (legal stripped)", len(got))
	}
	j := got[0]
	if j.Source != "remoteok" || j.ExternalID != "1234567" {
		t.Errorf("first job wrong: %+v", j)
	}
	if j.CompanyName != "Acme" || j.CompanySlug != "acme" {
		t.Errorf("company wrong: name=%q slug=%q", j.CompanyName, j.CompanySlug)
	}
	if !j.Remote {
		t.Errorf("remoteok jobs should always be remote")
	}
	if !strings.Contains(j.Description, "Build cool things") {
		t.Errorf("description: %q", j.Description)
	}
	if j.PostedAt == nil {
		t.Errorf("posted_at nil")
	}
}

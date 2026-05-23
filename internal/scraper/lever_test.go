package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const leverSample = `[
  {
    "id": "abc-123",
    "text": "Senior Backend Engineer",
    "hostedUrl": "https://jobs.lever.co/acme/abc-123",
    "applyUrl": "https://jobs.lever.co/acme/abc-123/apply",
    "categories": {
      "team": "Platform",
      "location": "Remote - Worldwide",
      "commitment": "Full-time"
    },
    "createdAt": 1697040000000,
    "descriptionPlain": "Build platform infrastructure in Go.",
    "description": "<p>Build platform infrastructure in Go.</p>",
    "lists": [
      {"text":"Responsibilities","content":"<ul><li>Own services</li></ul>"}
    ]
  },
  {
    "id": "def-456",
    "text": "Product Designer",
    "hostedUrl": "https://jobs.lever.co/acme/def-456",
    "categories": {
      "team": "Design",
      "location": "New York, NY",
      "commitment": "Full-time"
    },
    "createdAt": 1696000000000,
    "descriptionPlain": "Design things.",
    "description": "<p>Design things.</p>"
  }
]`

func TestLeverScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/postings/acme" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("mode") != "json" {
			t.Errorf("missing mode=json")
		}
		_, _ = w.Write([]byte(leverSample))
	}))
	defer srv.Close()

	lv := NewLever(srv.Client(), srv.URL, []string{"acme"})
	if lv.Name() != "lever" {
		t.Fatalf("name = %q", lv.Name())
	}
	got, err := lv.Scrape(context.Background(), ScrapeParams{})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	j := got[0]
	if j.Source != "lever" || j.ExternalID != "abc-123" || j.Title != "Senior Backend Engineer" {
		t.Errorf("first job wrong: %+v", j)
	}
	if j.Location != "Remote - Worldwide" || !j.Remote {
		t.Errorf("location/remote wrong: %q remote=%v", j.Location, j.Remote)
	}
	if !strings.Contains(j.Description, "Build platform infrastructure") {
		t.Errorf("description missing text: %q", j.Description)
	}
	if j.PostedAt == nil {
		t.Errorf("posted_at nil")
	}
	if got[1].Remote {
		t.Errorf("NYC job marked remote")
	}
}

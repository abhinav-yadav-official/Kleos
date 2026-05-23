package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const greenhouseSample = `{
  "jobs": [
    {
      "id": 4567890,
      "internal_job_id": 1234567,
      "title": "Senior Software Engineer",
      "absolute_url": "https://boards.greenhouse.io/acme/jobs/4567890",
      "location": {"name": "Remote - US"},
      "updated_at": "2026-04-10T14:30:00-04:00",
      "content": "&lt;p&gt;We are hiring a &lt;strong&gt;senior engineer&lt;/strong&gt; to build cool things.&lt;/p&gt;\n&lt;ul&gt;&lt;li&gt;Go&lt;/li&gt;&lt;li&gt;Postgres&lt;/li&gt;&lt;/ul&gt;"
    },
    {
      "id": 4567891,
      "title": "Product Manager",
      "absolute_url": "https://boards.greenhouse.io/acme/jobs/4567891",
      "location": {"name": "New York, NY"},
      "updated_at": "2026-04-09T10:00:00-04:00",
      "content": "&lt;p&gt;Lead product.&lt;/p&gt;"
    }
  ]
}`

func TestGreenhouseScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/boards/acme/jobs" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("content") != "true" {
			t.Errorf("missing content=true")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(greenhouseSample))
	}))
	defer srv.Close()

	gh := NewGreenhouse(srv.Client(), srv.URL, []string{"acme"})
	if gh.Name() != "greenhouse" {
		t.Fatalf("name = %q, want greenhouse", gh.Name())
	}

	got, err := gh.Scrape(context.Background(), ScrapeParams{})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	j := got[0]
	if j.Source != "greenhouse" {
		t.Errorf("source = %q", j.Source)
	}
	if j.ExternalID != "4567890" {
		t.Errorf("external_id = %q", j.ExternalID)
	}
	if j.Title != "Senior Software Engineer" {
		t.Errorf("title = %q", j.Title)
	}
	if j.URL != "https://boards.greenhouse.io/acme/jobs/4567890" {
		t.Errorf("url = %q", j.URL)
	}
	if j.Location != "Remote - US" {
		t.Errorf("location = %q", j.Location)
	}
	if !j.Remote {
		t.Errorf("expected remote=true for %q", j.Location)
	}
	if j.CompanySlug != "acme" {
		t.Errorf("company_slug = %q", j.CompanySlug)
	}
	if j.CompanyName != "acme" {
		t.Errorf("company_name = %q", j.CompanyName)
	}
	if !strings.Contains(j.Description, "senior engineer") {
		t.Errorf("description missing text, got: %q", j.Description)
	}
	if strings.Contains(j.Description, "<") || strings.Contains(j.Description, "&lt;") {
		t.Errorf("description still contains HTML/entities: %q", j.Description)
	}
	if j.PostedAt == nil || j.PostedAt.Year() != 2026 {
		t.Errorf("posted_at = %v", j.PostedAt)
	}
	if len(j.Raw) == 0 {
		t.Errorf("raw empty")
	}

	if got[1].Remote {
		t.Errorf("New York job marked remote")
	}
}

func TestGreenhouseSinceFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(greenhouseSample))
	}))
	defer srv.Close()

	gh := NewGreenhouse(srv.Client(), srv.URL, []string{"acme"})
	since := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	got, err := gh.Scrape(context.Background(), ScrapeParams{Since: since})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (only newer-than since)", len(got))
	}
	if got[0].ExternalID != "4567890" {
		t.Errorf("kept wrong job: %q", got[0].ExternalID)
	}
}

func TestGreenhouseHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	gh := NewGreenhouse(srv.Client(), srv.URL, []string{"acme"})
	_, err := gh.Scrape(context.Background(), ScrapeParams{})
	if err == nil {
		t.Fatal("expected error on 500")
	}
}

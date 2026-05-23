package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const ashbySample = `{
  "jobs": [
    {
      "id": "11111111-aaaa-bbbb-cccc-222222222222",
      "title": " Staff Engineer ",
      "department": "Engineering",
      "team": "Platform",
      "employmentType": "FullTime",
      "location": "Remote (US)",
      "publishedAt": "2026-04-10T14:30:00.753+00:00",
      "isListed": true,
      "isRemote": true,
      "workplaceType": "Remote",
      "descriptionHtml": "<p>Build core systems.</p>",
      "descriptionPlain": "Build core systems.",
      "jobUrl": "https://jobs.ashbyhq.com/acme/11111111"
    },
    {
      "id": "33333333-aaaa-bbbb-cccc-444444444444",
      "title": "Recruiter",
      "department": "People",
      "team": "Talent",
      "employmentType": "FullTime",
      "location": "San Francisco, CA",
      "publishedAt": "2026-04-09T12:00:00+00:00",
      "isListed": true,
      "isRemote": false,
      "workplaceType": "OnSite",
      "descriptionPlain": "Recruit people.",
      "jobUrl": "https://jobs.ashbyhq.com/acme/33333333"
    },
    {
      "id": "99999999-aaaa-bbbb-cccc-555555555555",
      "title": "Hidden Role",
      "isListed": false,
      "publishedAt": "2026-04-08T12:00:00+00:00",
      "jobUrl": "https://jobs.ashbyhq.com/acme/99999999"
    }
  ]
}`

func TestAshbyScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/posting-api/job-board/acme" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("includeCompensation") != "false" {
			t.Errorf("missing includeCompensation=false")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ashbySample))
	}))
	defer srv.Close()

	ab := NewAshby(srv.Client(), srv.URL, []string{"acme"})
	if ab.Name() != "ashby" {
		t.Fatalf("name = %q", ab.Name())
	}
	got, err := ab.Scrape(context.Background(), ScrapeParams{})
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (isListed=false dropped)", len(got))
	}
	j := got[0]
	if j.Source != "ashby" || j.ExternalID != "11111111-aaaa-bbbb-cccc-222222222222" {
		t.Errorf("first job wrong: %+v", j)
	}
	if j.Title != "Staff Engineer" {
		t.Errorf("title not trimmed: %q", j.Title)
	}
	if !j.Remote || j.Location != "Remote (US)" {
		t.Errorf("remote/loc wrong: %v %q", j.Remote, j.Location)
	}
	if j.PostedAt == nil || j.PostedAt.Year() != 2026 {
		t.Errorf("posted_at wrong: %v", j.PostedAt)
	}
	if !strings.Contains(j.Description, "Build core systems") {
		t.Errorf("description: %q", j.Description)
	}
	if got[1].Remote {
		t.Errorf("SF job marked remote")
	}
}

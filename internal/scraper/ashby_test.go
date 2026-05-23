package scraper

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const ashbySample = `{
  "jobs": [
    {
      "id": "11111111-aaaa-bbbb-cccc-222222222222",
      "title": "Staff Engineer",
      "departmentName": "Platform",
      "locationName": "Remote (US)",
      "employmentType": "FullTime",
      "publishedDate": "2026-04-10T14:30:00Z",
      "isRemote": true,
      "descriptionHtml": "<p>Build core systems.</p>",
      "descriptionPlain": "Build core systems.",
      "jobUrl": "https://jobs.ashbyhq.com/acme/11111111"
    },
    {
      "id": "33333333-aaaa-bbbb-cccc-444444444444",
      "title": "Recruiter",
      "departmentName": "People",
      "locationName": "San Francisco, CA",
      "employmentType": "FullTime",
      "publishedDate": "2026-04-09T12:00:00Z",
      "isRemote": false,
      "descriptionPlain": "Recruit people.",
      "jobUrl": "https://jobs.ashbyhq.com/acme/33333333"
    }
  ]
}`

func TestAshbyScrape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/posting-api/job-board/acme" {
			t.Errorf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		if got["includeCompensation"] != false {
			t.Errorf("body missing includeCompensation=false: %v", got)
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
		t.Fatalf("len = %d, want 2", len(got))
	}
	j := got[0]
	if j.Source != "ashby" || j.ExternalID != "11111111-aaaa-bbbb-cccc-222222222222" {
		t.Errorf("first job wrong: %+v", j)
	}
	if !j.Remote || j.Location != "Remote (US)" {
		t.Errorf("remote/loc wrong: %v %q", j.Remote, j.Location)
	}
	if !strings.Contains(j.Description, "Build core systems") {
		t.Errorf("description: %q", j.Description)
	}
	if got[1].Remote {
		t.Errorf("SF job marked remote")
	}
}

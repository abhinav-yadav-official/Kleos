package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultAshbyBase = "https://api.ashbyhq.com"

type Ashby struct {
	client  *http.Client
	baseURL string
	slugs   []string
}

func NewAshby(client *http.Client, baseURL string, slugs []string) *Ashby {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultAshbyBase
	}
	return &Ashby{client: client, baseURL: strings.TrimRight(baseURL, "/"), slugs: slugs}
}

func (a *Ashby) Name() string { return "ashby" }

type ashbyResp struct {
	Jobs []ashbyJob `json:"jobs"`
}

type ashbyJob struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	DepartmentName   string `json:"departmentName"`
	LocationName     string `json:"locationName"`
	EmploymentType   string `json:"employmentType"`
	PublishedDate    string `json:"publishedDate"`
	IsRemote         bool   `json:"isRemote"`
	DescriptionHTML  string `json:"descriptionHtml"`
	DescriptionPlain string `json:"descriptionPlain"`
	JobURL           string `json:"jobUrl"`
}

func (a *Ashby) Scrape(ctx context.Context, p ScrapeParams) ([]ScrapedJob, error) {
	var out []ScrapedJob
	for _, slug := range a.slugs {
		jobs, err := a.scrapeBoard(ctx, slug, p)
		if err != nil {
			return nil, fmt.Errorf("ashby %s: %w", slug, err)
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func (a *Ashby) scrapeBoard(ctx context.Context, slug string, p ScrapeParams) ([]ScrapedJob, error) {
	url := fmt.Sprintf("%s/posting-api/job-board/%s", a.baseURL, slug)
	body, _ := json.Marshal(map[string]bool{"includeCompensation": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kleos-jobscraper/0.1")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rb, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(rb), 200))
	}

	var parsed ashbyResp
	if err := json.Unmarshal(rb, &parsed); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	var rawWrap struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	_ = json.Unmarshal(rb, &rawWrap)

	out := make([]ScrapedJob, 0, len(parsed.Jobs))
	for i, j := range parsed.Jobs {
		posted := parseTime(j.PublishedDate)
		if !p.Since.IsZero() && posted != nil && posted.Before(p.Since) {
			continue
		}
		desc := j.DescriptionPlain
		if desc == "" {
			desc = cleanHTML(j.DescriptionHTML)
		}
		var raw json.RawMessage
		if i < len(rawWrap.Jobs) {
			raw = rawWrap.Jobs[i]
		}
		out = append(out, ScrapedJob{
			Source:      "ashby",
			ExternalID:  j.ID,
			CompanyName: slug,
			CompanySlug: slug,
			Title:       j.Title,
			Description: strings.TrimSpace(desc),
			Location:    j.LocationName,
			Remote:      j.IsRemote || isRemote(j.LocationName),
			URL:         j.JobURL,
			PostedAt:    posted,
			Raw:         raw,
		})
	}
	return out, nil
}

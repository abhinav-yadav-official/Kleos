package scraper

import (
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
	Department       string `json:"department"`
	Team             string `json:"team"`
	EmploymentType   string `json:"employmentType"`
	Location         string `json:"location"`
	PublishedAt      string `json:"publishedAt"`
	IsListed         bool   `json:"isListed"`
	IsRemote         bool   `json:"isRemote"`
	WorkplaceType    string `json:"workplaceType"`
	JobURL           string `json:"jobUrl"`
	ApplyURL         string `json:"applyUrl"`
	DescriptionHTML  string `json:"descriptionHtml"`
	DescriptionPlain string `json:"descriptionPlain"`
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
	url := fmt.Sprintf("%s/posting-api/job-board/%s?includeCompensation=false", a.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
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
		if !j.IsListed {
			continue
		}
		posted := parseTime(j.PublishedAt)
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
		title := strings.TrimSpace(j.Title)
		out = append(out, ScrapedJob{
			Source:      "ashby",
			ExternalID:  j.ID,
			CompanyName: slug,
			CompanySlug: slug,
			Title:       title,
			Description: strings.TrimSpace(desc),
			Location:    j.Location,
			Remote:      j.IsRemote || isRemote(j.Location) || strings.EqualFold(j.WorkplaceType, "Remote"),
			URL:         j.JobURL,
			PostedAt:    posted,
			Raw:         raw,
		})
	}
	return out, nil
}

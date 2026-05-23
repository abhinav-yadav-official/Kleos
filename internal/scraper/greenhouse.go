package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DefaultGreenhouseBase = "https://boards-api.greenhouse.io"

type Greenhouse struct {
	client  *http.Client
	baseURL string
	slugs   []string
}

func NewGreenhouse(client *http.Client, baseURL string, slugs []string) *Greenhouse {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultGreenhouseBase
	}
	return &Greenhouse{client: client, baseURL: strings.TrimRight(baseURL, "/"), slugs: slugs}
}

func (g *Greenhouse) Name() string { return "greenhouse" }

type ghResponse struct {
	Jobs []ghJob `json:"jobs"`
}

type ghJob struct {
	ID            int64           `json:"id"`
	InternalJobID int64           `json:"internal_job_id"`
	Title         string          `json:"title"`
	AbsoluteURL   string          `json:"absolute_url"`
	Location      ghLocation      `json:"location"`
	UpdatedAt     string          `json:"updated_at"`
	Content       string          `json:"content"`
	Raw           json.RawMessage `json:"-"`
}

type ghLocation struct {
	Name string `json:"name"`
}

func (g *Greenhouse) Scrape(ctx context.Context, p ScrapeParams) ([]ScrapedJob, error) {
	var out []ScrapedJob
	for _, slug := range g.slugs {
		jobs, err := g.scrapeBoard(ctx, slug, p)
		if err != nil {
			return nil, fmt.Errorf("greenhouse %s: %w", slug, err)
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func (g *Greenhouse) scrapeBoard(ctx context.Context, slug string, p ScrapeParams) ([]ScrapedJob, error) {
	url := fmt.Sprintf("%s/v1/boards/%s/jobs?content=true", g.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kleos-jobscraper/0.1")

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var parsed ghResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	// Preserve raw per job.
	var rawWrap struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	_ = json.Unmarshal(body, &rawWrap)

	out := make([]ScrapedJob, 0, len(parsed.Jobs))
	for i, j := range parsed.Jobs {
		posted := parseTime(j.UpdatedAt)
		if !p.Since.IsZero() && posted != nil && posted.Before(p.Since) {
			continue
		}
		var raw json.RawMessage
		if i < len(rawWrap.Jobs) {
			raw = rawWrap.Jobs[i]
		}
		desc := cleanHTML(j.Content)
		loc := j.Location.Name
		out = append(out, ScrapedJob{
			Source:      "greenhouse",
			ExternalID:  strconv.FormatInt(j.ID, 10),
			CompanyName: slug,
			CompanySlug: slug,
			Title:       j.Title,
			Description: desc,
			Location:    loc,
			Remote:      isRemote(loc),
			URL:         j.AbsoluteURL,
			PostedAt:    posted,
			Raw:         raw,
		})
	}
	return out, nil
}

var tagRE = regexp.MustCompile(`<[^>]+>`)
var wsRE = regexp.MustCompile(`[ \t]+`)

func cleanHTML(s string) string {
	s = html.UnescapeString(s)
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\r", "")
	s = wsRE.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	s = strings.Join(lines, "\n")
	// collapse repeated blank lines
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

func isRemote(loc string) bool {
	l := strings.ToLower(loc)
	return strings.Contains(l, "remote") || strings.Contains(l, "anywhere") || strings.Contains(l, "worldwide")
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05-07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return &t
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

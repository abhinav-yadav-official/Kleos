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

const DefaultLeverBase = "https://api.lever.co"

type Lever struct {
	client  *http.Client
	baseURL string
	slugs   []string
}

func NewLever(client *http.Client, baseURL string, slugs []string) *Lever {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultLeverBase
	}
	return &Lever{client: client, baseURL: strings.TrimRight(baseURL, "/"), slugs: slugs}
}

func (l *Lever) Name() string { return "lever" }

type leverPosting struct {
	ID               string            `json:"id"`
	Text             string            `json:"text"`
	HostedURL        string            `json:"hostedUrl"`
	Categories       leverCats         `json:"categories"`
	CreatedAt        int64             `json:"createdAt"`
	DescriptionPlain string            `json:"descriptionPlain"`
	Description      string            `json:"description"`
	Lists            []leverListBlock  `json:"lists"`
}

type leverCats struct {
	Team       string `json:"team"`
	Location   string `json:"location"`
	Commitment string `json:"commitment"`
}

type leverListBlock struct {
	Text    string `json:"text"`
	Content string `json:"content"`
}

func (l *Lever) Scrape(ctx context.Context, p ScrapeParams) ([]ScrapedJob, error) {
	var out []ScrapedJob
	for _, slug := range l.slugs {
		jobs, err := l.scrapeBoard(ctx, slug, p)
		if err != nil {
			return nil, fmt.Errorf("lever %s: %w", slug, err)
		}
		out = append(out, jobs...)
	}
	return out, nil
}

func (l *Lever) scrapeBoard(ctx context.Context, slug string, p ScrapeParams) ([]ScrapedJob, error) {
	url := fmt.Sprintf("%s/v0/postings/%s?mode=json", l.baseURL, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kleos-jobscraper/0.1")

	resp, err := l.client.Do(req)
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

	var postings []leverPosting
	if err := json.Unmarshal(body, &postings); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	var rawArr []json.RawMessage
	_ = json.Unmarshal(body, &rawArr)

	out := make([]ScrapedJob, 0, len(postings))
	for i, ps := range postings {
		posted := timeFromMillis(ps.CreatedAt)
		if !p.Since.IsZero() && posted != nil && posted.Before(p.Since) {
			continue
		}
		var raw json.RawMessage
		if i < len(rawArr) {
			raw = rawArr[i]
		}
		desc := ps.DescriptionPlain
		if desc == "" {
			desc = cleanHTML(ps.Description)
		}
		for _, blk := range ps.Lists {
			if blk.Text != "" {
				desc += "\n\n" + blk.Text + ":\n"
			}
			desc += cleanHTML(blk.Content)
		}
		out = append(out, ScrapedJob{
			Source:      "lever",
			ExternalID:  ps.ID,
			CompanyName: slug,
			CompanySlug: slug,
			Title:       ps.Text,
			Description: strings.TrimSpace(desc),
			Location:    ps.Categories.Location,
			Remote:      isRemote(ps.Categories.Location),
			URL:         ps.HostedURL,
			PostedAt:    posted,
			Raw:         raw,
		})
	}
	return out, nil
}

func timeFromMillis(ms int64) *time.Time {
	if ms == 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}

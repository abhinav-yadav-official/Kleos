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

const DefaultRemoteOKBase = "https://remoteok.com"

type RemoteOK struct {
	client  *http.Client
	baseURL string
}

func NewRemoteOK(client *http.Client, baseURL string) *RemoteOK {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultRemoteOKBase
	}
	return &RemoteOK{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

func (r *RemoteOK) Name() string { return "remoteok" }

type remoteOKJob struct {
	ID          json.RawMessage `json:"id"`
	Slug        string          `json:"slug"`
	Company     string          `json:"company"`
	Position    string          `json:"position"`
	Description string          `json:"description"`
	Location    string          `json:"location"`
	URL         string          `json:"url"`
	Date        string          `json:"date"`
	Epoch       int64           `json:"epoch"`
}

func (r *RemoteOK) Scrape(ctx context.Context, p ScrapeParams) ([]ScrapedJob, error) {
	url := r.baseURL + "/api"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "kleos-jobscraper/0.1 (+https://abhiyadav.in/kleos)")

	resp, err := r.client.Do(req)
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

	var entries []json.RawMessage
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	out := make([]ScrapedJob, 0, len(entries))
	for _, raw := range entries {
		var j remoteOKJob
		if err := json.Unmarshal(raw, &j); err != nil {
			continue
		}
		if len(j.ID) == 0 {
			// Legal/metadata entry.
			continue
		}
		id := strings.Trim(string(j.ID), `"`)
		var posted *time.Time
		if j.Epoch > 0 {
			t := time.Unix(j.Epoch, 0).UTC()
			posted = &t
		} else if j.Date != "" {
			posted = parseTime(j.Date)
		}
		if !p.Since.IsZero() && posted != nil && posted.Before(p.Since) {
			continue
		}
		companyName := strings.TrimSpace(j.Company)
		companySlug := NormalizeSlug(companyName)
		if companySlug == "" {
			// Non-Latin company name (e.g. CJK); fall back to a stable
			// per-job slug so the row still persists.
			companySlug = "remoteok-" + id
			if companyName == "" {
				companyName = companySlug
			}
		}
		out = append(out, ScrapedJob{
			Source:      "remoteok",
			ExternalID:  id,
			CompanyName: companyName,
			CompanySlug: companySlug,
			Title:       j.Position,
			Description: cleanHTML(j.Description),
			Location:    j.Location,
			Remote:      true,
			URL:         j.URL,
			PostedAt:    posted,
			Raw:         raw,
		})
	}
	return out, nil
}

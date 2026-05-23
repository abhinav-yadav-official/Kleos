package scraper

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Fragile sources: their public surfaces have no stable API and routinely
// trigger anti-bot defences. The implementations here exist so the worker
// CLI can name them, attempt a single probe, and skip cleanly on any failure
// rather than crash the queue. Plan §8 marks all four as best-effort.

type fragile struct {
	name string
	url  string
	client *http.Client
}

func (f *fragile) Name() string { return f.name }

func (f *fragile) Scrape(ctx context.Context, _ ScrapeParams) ([]ScrapedJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.url, nil)
	if err != nil {
		slog.Warn("fragile source: build request failed", "source", f.name, "error", err)
		return nil, nil
	}
	req.Header.Set("User-Agent", "kleos-jobscraper/0.1")
	resp, err := f.client.Do(req)
	if err != nil {
		slog.Warn("fragile source: request failed, skipping", "source", f.name, "error", err)
		return nil, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		slog.Warn("fragile source: non-2xx, skipping", "source", f.name, "status", resp.StatusCode)
		return nil, nil
	}
	// v1 returns no parsed jobs from fragile sources. Stable parsing of HTML/RSS
	// for Wellfound/Indeed/Naukri/LinkedIn is deferred; a parser breakage would
	// poison the matching pipeline, so the probe just confirms reachability.
	slog.Info("fragile source reachable but parser deferred", "source", f.name, "status", resp.StatusCode)
	return nil, nil
}

func newFragile(name, url string, client *http.Client) *fragile {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &fragile{name: name, url: url, client: client}
}

func NewWellfound(client *http.Client) Scraper {
	return newFragile("wellfound", "https://wellfound.com/jobs", client)
}

func NewIndeed(client *http.Client) Scraper {
	return newFragile("indeed", "https://www.indeed.com/rss?q=software+engineer", client)
}

func NewNaukri(client *http.Client) Scraper {
	return newFragile("naukri", "https://www.naukri.com/jobs-in-india", client)
}

func NewLinkedIn(client *http.Client) Scraper {
	return newFragile("linkedin", "https://www.linkedin.com/jobs/search/?keywords=software", client)
}

package scraper

import (
	"context"
	"encoding/json"
	"time"
)

type Scraper interface {
	Name() string
	Scrape(ctx context.Context, p ScrapeParams) ([]ScrapedJob, error)
}

type ScrapeParams struct {
	Titles     []string
	Locations  []string
	Keywords   []string
	RemoteOnly bool
	Since      time.Time
}

type ScrapedJob struct {
	Source        string
	ExternalID    string
	CompanyName   string
	CompanyDomain string
	CompanySlug   string
	Title         string
	Description   string
	Location      string
	Remote        bool
	URL           string
	PostedAt      *time.Time
	Raw           json.RawMessage
}

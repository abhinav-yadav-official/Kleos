package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
	"github.com/abhinav-yadav-official/Kleos/internal/scraper"
)

// One-shot job scraper CLI.
//
// Usage:
//
//	worker-jobscraper --source greenhouse --slugs gitlab,sourcegraph
//	worker-jobscraper --source remoteok
//
// Per-source rate limits are applied automatically (see scraper.SourceLimits).
func main() {
	source := flag.String("source", "greenhouse", "scraper source: greenhouse|lever|ashby|remoteok")
	slugs := flag.String("slugs", "", "comma-separated board slugs (required for greenhouse/lever/ashby)")
	sinceStr := flag.String("since", "", "only keep jobs posted after this RFC3339 timestamp (optional)")
	flag.Parse()

	slugList := splitCSV(*slugs)

	var since time.Time
	if *sinceStr != "" {
		t, err := time.Parse(time.RFC3339, *sinceStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --since: %v\n", err)
			os.Exit(2)
		}
		since = t
	}

	cfg := config.Load()
	ctx := context.Background()

	pg, err := db.ConnectPostgres(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	client := scraper.NewLimitedHTTPClient(*source)

	var sc scraper.Scraper
	switch *source {
	case "greenhouse":
		if len(slugList) == 0 {
			fmt.Fprintln(os.Stderr, "missing --slugs for greenhouse")
			os.Exit(2)
		}
		sc = scraper.NewGreenhouse(client, scraper.DefaultGreenhouseBase, slugList)
	case "lever":
		if len(slugList) == 0 {
			fmt.Fprintln(os.Stderr, "missing --slugs for lever")
			os.Exit(2)
		}
		sc = scraper.NewLever(client, scraper.DefaultLeverBase, slugList)
	case "ashby":
		if len(slugList) == 0 {
			fmt.Fprintln(os.Stderr, "missing --slugs for ashby")
			os.Exit(2)
		}
		sc = scraper.NewAshby(client, scraper.DefaultAshbyBase, slugList)
	case "remoteok":
		sc = scraper.NewRemoteOK(client, scraper.DefaultRemoteOKBase)
	case "wellfound":
		sc = scraper.NewWellfound(client)
	case "indeed":
		sc = scraper.NewIndeed(client)
	case "naukri":
		sc = scraper.NewNaukri(client)
	case "linkedin":
		sc = scraper.NewLinkedIn(client)
	default:
		fmt.Fprintf(os.Stderr, "unsupported source: %s\n", *source)
		os.Exit(2)
	}

	jobs, err := sc.Scrape(ctx, scraper.ScrapeParams{Since: since})
	if err != nil {
		slog.Error("scrape", "source", *source, "error", err)
		os.Exit(1)
	}

	store := scraper.NewStore(pg.Pool())
	inserted, updated, err := store.PersistAll(ctx, jobs)
	if err != nil {
		slog.Error("persist", "error", err)
		os.Exit(1)
	}

	slog.Info("scrape complete",
		"source", *source,
		"slugs", slugList,
		"scraped", len(jobs),
		"inserted", inserted,
		"updated", updated,
	)

	if errors.Is(ctx.Err(), context.Canceled) {
		os.Exit(1)
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

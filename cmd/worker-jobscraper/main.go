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

// One-shot job scraper CLI for v1.
//
// Usage:
//
//	worker-jobscraper --source greenhouse --slugs gitlab,sourcegraph
//
// Subsequent slices add cron + asynq. For now this is invoked manually (or via
// systemd timer / cron) to populate the jobs table.
func main() {
	source := flag.String("source", "greenhouse", "scraper source (greenhouse)")
	slugs := flag.String("slugs", "", "comma-separated board slugs")
	sinceStr := flag.String("since", "", "only keep jobs posted after this RFC3339 timestamp (optional)")
	flag.Parse()

	if *slugs == "" {
		fmt.Fprintln(os.Stderr, "missing --slugs")
		os.Exit(2)
	}
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

	var sc scraper.Scraper
	switch *source {
	case "greenhouse":
		sc = scraper.NewGreenhouse(nil, scraper.DefaultGreenhouseBase, slugList)
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

	if len(jobs) == 0 {
		os.Exit(0)
	}
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

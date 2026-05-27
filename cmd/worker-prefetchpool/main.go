package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
	"github.com/abhinav-yadav-official/Kleos/internal/emailfinder"
)

// Reads a JSON seed file of company metadata, upserts each into companies with
// a country tag, and runs the email-finder strategy chain over them so
// recruiters land in the shared pool. Designed to run periodically (cron) or
// ad-hoc from the VPS to refresh the country-specific recruiter pool.
//
// JSON shape:
//   [{ "slug": "razorpay", "name": "Razorpay", "domain": "razorpay.com",
//      "careers_url": "...", "github_org": "razorpay" }, ...]
//
// Flags:
//   --seed   path to JSON seed file (required)
//   --country ISO 3166-1 alpha-2 code stored on each upserted company (default IN)
type seedRow struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Domain     string `json:"domain"`
	CareersURL string `json:"careers_url"`
	GitHubOrg  string `json:"github_org"`
}

func main() {
	seedPath := flag.String("seed", "", "path to JSON seed file")
	country := flag.String("country", "IN", "country tag to stamp on each company")
	limit := flag.Int("limit", 0, "max companies to process per run (0 = all)")
	flag.Parse()
	if *seedPath == "" {
		slog.Error("missing --seed")
		os.Exit(2)
	}

	raw, err := os.ReadFile(*seedPath)
	if err != nil {
		slog.Error("read seed", "error", err)
		os.Exit(1)
	}
	var rows []seedRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		slog.Error("parse seed", "error", err)
		os.Exit(1)
	}
	if *limit > 0 && *limit < len(rows) {
		rows = rows[:*limit]
	}

	cfg := config.Load()
	ctx := context.Background()
	pg, err := db.ConnectPostgres(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	httpClient := &http.Client{Timeout: 20 * time.Second}
	mailto := emailfinder.NewMailtoExtractor(httpClient)
	github := emailfinder.NewGitHubMiner(httpClient, emailfinder.DefaultGitHubBase, os.Getenv("GITHUB_TOKEN"))
	svc := emailfinder.NewService(pg.Pool(), mailto, github)

	upserted, foundRows := 0, 0
	for _, row := range rows {
		id, err := svc.UpsertCompanyMetadata(ctx, row.Slug, row.Name, row.CareersURL, row.Domain, row.GitHubOrg, *country)
		if err != nil {
			slog.Warn("upsert company", "slug", row.Slug, "error", err)
			continue
		}
		upserted++
		n, _, err := svc.FindForCompany(ctx, id)
		if err != nil {
			slog.Warn("find recruiters", "slug", row.Slug, "error", err)
		}
		foundRows += n
		slog.Info("seeded", "slug", row.Slug, "company_id", id, "recruiters_added", n)
	}
	slog.Info("prefetchpool run complete", "country", *country, "companies", upserted, "recruiters_added", foundRows)
}

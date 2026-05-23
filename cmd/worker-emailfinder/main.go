package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
	"github.com/abhinav-yadav-official/Kleos/internal/emailfinder"
)

// One-shot email finder: walks campaign_matches in state='new' for active
// campaigns, transitions them through finding_email → email_found|email_missing
// while populating the recruiters table from mailto + github strategies.
func main() {
	limit := flag.Int("limit", 50, "max matches to process per run")
	flag.Parse()

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

	rows, err := pg.Pool().Query(ctx, `
		SELECT m.id::text, j.company_id::text
		FROM campaign_matches m
		JOIN jobs j ON j.id = m.job_id
		JOIN campaigns c ON c.id = m.campaign_id
		WHERE m.state = 'new' AND c.status = 'active' AND j.company_id IS NOT NULL
		ORDER BY m.match_score DESC, m.matched_at DESC
		LIMIT $1
	`, *limit)
	if err != nil {
		slog.Error("query matches", "error", err)
		os.Exit(1)
	}
	type matchRow struct {
		ID        string
		CompanyID string
	}
	var matches []matchRow
	for rows.Next() {
		var m matchRow
		if err := rows.Scan(&m.ID, &m.CompanyID); err != nil {
			rows.Close()
			slog.Error("scan match", "error", err)
			os.Exit(1)
		}
		matches = append(matches, m)
	}
	rows.Close()

	processed, found, missing, inserted := 0, 0, 0, 0
	for _, m := range matches {
		if _, err := pg.Pool().Exec(ctx,
			`UPDATE campaign_matches SET state='finding_email' WHERE id=$1::uuid AND state='new'`,
			m.ID); err != nil {
			slog.Error("set finding_email", "match_id", m.ID, "error", err)
			continue
		}

		n, _, err := svc.FindForCompany(ctx, m.CompanyID)
		if err != nil {
			slog.Warn("find recruiters", "company_id", m.CompanyID, "error", err)
		}
		inserted += n

		has, err := svc.HasAnyRecruiter(ctx, m.CompanyID)
		if err != nil {
			slog.Error("check recruiters", "company_id", m.CompanyID, "error", err)
			continue
		}
		next := "email_missing"
		if has {
			next = "email_found"
		}
		if _, err := pg.Pool().Exec(ctx,
			`UPDATE campaign_matches SET state=$2 WHERE id=$1::uuid`, m.ID, next); err != nil {
			slog.Error("transition", "match_id", m.ID, "state", next, "error", err)
			continue
		}
		processed++
		if next == "email_found" {
			found++
		} else {
			missing++
		}
	}

	slog.Info("emailfinder run complete",
		"considered", len(matches),
		"processed", processed,
		"email_found", found,
		"email_missing", missing,
		"recruiters_inserted", inserted,
	)

	if errors.Is(ctx.Err(), context.Canceled) {
		os.Exit(1)
	}
}

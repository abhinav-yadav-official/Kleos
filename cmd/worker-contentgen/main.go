package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	"github.com/abhinav-yadav-official/Kleos/internal/contentgen"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
)

// One-shot content generator: walks campaign_matches in state='email_found'
// for active campaigns, transitions to 'generating', invokes Codex, persists
// 3 email_drafts, and lands at 'generated' or 'failed' (reason=content_quality
// or codex error).
func main() {
	limit := flag.Int("limit", 10, "max matches to process per run")
	codexCmd := flag.String("codex-cmd", "codex", "Codex CLI command")
	flag.Parse()

	timeout := 60 * time.Second
	if v := os.Getenv("CODEX_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			timeout = d
		}
	}

	cfg := config.Load()
	ctx := context.Background()

	pg, err := db.ConnectPostgres(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	gen := contentgen.NewCLICodex(*codexCmd, timeout)
	svc := contentgen.NewService(pg.Pool(), gen)

	rows, err := pg.Pool().Query(ctx, `
		SELECT m.id::text
		FROM campaign_matches m
		JOIN campaigns c ON c.id = m.campaign_id
		WHERE m.state = 'email_found' AND c.status = 'active'
		ORDER BY m.match_score DESC, m.matched_at DESC
		LIMIT $1
	`, *limit)
	if err != nil {
		slog.Error("query matches", "error", err)
		os.Exit(1)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			slog.Error("scan", "error", err)
			os.Exit(1)
		}
		ids = append(ids, id)
	}
	rows.Close()

	generated, failed, skipped := 0, 0, 0
	for _, id := range ids {
		if _, err := pg.Pool().Exec(ctx,
			`UPDATE campaign_matches SET state='generating' WHERE id=$1::uuid AND state='email_found'`,
			id); err != nil {
			slog.Error("set generating", "match_id", id, "error", err)
			continue
		}
		state, draftID, err := svc.GenerateOne(ctx, id)
		switch {
		case errors.Is(err, contentgen.ErrNoRecruiter):
			// Should not happen — emailfinder gates entry, but stay safe.
			slog.Warn("no recruiter for match", "match_id", id)
			_, _ = pg.Pool().Exec(ctx,
				`UPDATE campaign_matches SET state='email_missing' WHERE id=$1::uuid`, id)
			skipped++
			continue
		case errors.Is(err, contentgen.ErrContentQuality):
			slog.Warn("content quality failure", "match_id", id)
			_, _ = pg.Pool().Exec(ctx,
				`UPDATE campaign_matches SET state='failed' WHERE id=$1::uuid`, id)
			failed++
			continue
		case err != nil:
			slog.Error("generate", "match_id", id, "error", err)
			_, _ = pg.Pool().Exec(ctx,
				`UPDATE campaign_matches SET state='failed' WHERE id=$1::uuid`, id)
			failed++
			continue
		}
		if state == "generated" {
			if _, err := pg.Pool().Exec(ctx,
				`UPDATE campaign_matches SET state='generated' WHERE id=$1::uuid`, id); err != nil {
				slog.Error("set generated", "match_id", id, "error", err)
				continue
			}
			generated++
			slog.Info("generated drafts", "match_id", id, "chosen_draft_id", draftID)
		}
	}
	slog.Info("contentgen run complete",
		"considered", len(ids),
		"generated", generated,
		"failed", failed,
		"skipped", skipped,
	)

	if errors.Is(ctx.Err(), context.Canceled) {
		os.Exit(1)
	}
}

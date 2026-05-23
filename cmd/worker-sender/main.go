package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	appcrypto "github.com/abhinav-yadav-official/Kleos/internal/crypto"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
	"github.com/abhinav-yadav-official/Kleos/internal/sender"
)

// One-shot sender: walks campaign_matches in state='generated' for active
// campaigns, transitions to 'queued', applies the §11 jitter window, sends via
// SMTP, and lands the match at 'sent' / 'failed' / 'skipped'.
func main() {
	limit := flag.Int("limit", 5, "max matches to attempt per run")
	jitterMin := flag.Int("jitter-min", sender.DefaultJitterMinSec, "min pre-send jitter seconds")
	jitterMax := flag.Int("jitter-max", sender.DefaultJitterMaxSec, "max pre-send jitter seconds")
	skipJitter := flag.Bool("skip-jitter", false, "disable pre-send jitter (testing only)")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pg, err := db.ConnectPostgres(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	codec, err := appcrypto.NewAESGCM(cfg.SMTPKey)
	if err != nil {
		slog.Error("smtp codec", "error", err)
		os.Exit(1)
	}
	svc := sender.NewService(pg.Pool(), codec, sender.NewNetSMTPTransport())

	rows, err := pg.Pool().Query(ctx, `
		SELECT m.id::text
		FROM campaign_matches m
		JOIN campaigns c ON c.id = m.campaign_id
		WHERE m.state = 'generated' AND c.status = 'active'
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

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	sent, failed, skipped, queued := 0, 0, 0, 0
	for _, id := range ids {
		if _, err := pg.Pool().Exec(ctx,
			`UPDATE campaign_matches SET state='queued' WHERE id=$1::uuid AND state='generated'`,
			id); err != nil {
			slog.Error("set queued", "match_id", id, "error", err)
			continue
		}
		if !*skipJitter && *jitterMax > 0 {
			delay := *jitterMin
			if *jitterMax > *jitterMin {
				delay += rng.Intn(*jitterMax - *jitterMin)
			}
			slog.Info("jitter sleep", "match_id", id, "seconds", delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}
		outcome := svc.SendOne(ctx, id)
		switch outcome.State {
		case "sent":
			sent++
			slog.Info("sent", "match_id", id, "message_id", outcome.MessageID)
		case "failed":
			failed++
			slog.Warn("send failed", "match_id", id, "reason", outcome.Reason, "class", outcome.Class.String(), "error", outcome.Err)
		case "skipped":
			skipped++
			slog.Info("skipped", "match_id", id, "reason", outcome.Reason)
		case "queued":
			queued++
			slog.Info("left queued", "match_id", id, "reason", outcome.Reason)
		}
	}
	slog.Info("sender run complete",
		"considered", len(ids), "sent", sent, "failed", failed, "skipped", skipped, "queued", queued)

	if errors.Is(ctx.Err(), context.Canceled) {
		os.Exit(1)
	}
}

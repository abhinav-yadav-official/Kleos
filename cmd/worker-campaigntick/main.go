package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/almostturingcomplete/Kleos/internal/campaigns"
	"github.com/almostturingcomplete/Kleos/internal/config"
	"github.com/almostturingcomplete/Kleos/internal/db"
)

// One-shot campaign tick: for every active campaign, score unmatched jobs
// against the user's preferences and insert campaign_matches rows.
func main() {
	cfg := config.Load()
	ctx := context.Background()

	pg, err := db.ConnectPostgres(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	inserted, err := campaigns.Tick(ctx, pg.Pool())
	if err != nil {
		slog.Error("tick failed", "error", err)
		os.Exit(1)
	}
	slog.Info("tick complete", "new_matches", inserted)
}

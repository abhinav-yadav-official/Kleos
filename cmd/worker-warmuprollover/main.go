package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
	"github.com/abhinav-yadav-official/Kleos/internal/sender"
)

// One-shot daily warmup rollover: for every non-paused warmup_state row whose
// last_rollover is before today, advance current_day by 1, recompute
// todays_limit per §11, and reset todays_sent to 0. Intended to be invoked by
// systemd timer / cron at 00:05 UTC.
func main() {
	cfg := config.Load()
	ctx := context.Background()
	pg, err := db.ConnectPostgres(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	svc := sender.NewService(pg.Pool(), nil, nil)
	n, err := svc.Rollover(ctx)
	if err != nil {
		slog.Error("rollover", "error", err)
		os.Exit(1)
	}
	slog.Info("warmup rollover complete", "rows_advanced", n)
}

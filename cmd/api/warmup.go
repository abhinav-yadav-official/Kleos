package main

import (
	"context"
	"errors"

	apphttp "github.com/abhinav-yadav-official/Kleos/internal/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type warmupAdapter struct {
	pool *pgxpool.Pool
}

func newWarmupAdapter(pool *pgxpool.Pool) *warmupAdapter {
	return &warmupAdapter{pool: pool}
}

func (a *warmupAdapter) Get(ctx context.Context, userID string) (apphttp.WarmupState, error) {
	var s apphttp.WarmupState
	var notes *string
	err := a.pool.QueryRow(ctx, `
		SELECT user_id::text, smtp_id::text, start_date, current_day, todays_sent, todays_limit,
			last_rollover, paused, notes
		FROM warmup_state WHERE user_id = $1::uuid
	`, userID).Scan(&s.UserID, &s.SMTPID, &s.StartDate, &s.CurrentDay, &s.TodaysSent, &s.TodaysLimit,
		&s.LastRollover, &s.Paused, &notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return apphttp.WarmupState{}, apphttp.ErrWarmupNotFound
	}
	if err != nil {
		return apphttp.WarmupState{}, err
	}
	if notes != nil {
		s.Notes = *notes
	}
	return s, nil
}

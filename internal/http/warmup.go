package apphttp

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type WarmupState struct {
	UserID       string    `json:"user_id"`
	SMTPID       string    `json:"smtp_id"`
	StartDate    time.Time `json:"start_date"`
	CurrentDay   int       `json:"current_day"`
	TodaysSent   int       `json:"todays_sent"`
	TodaysLimit  int       `json:"todays_limit"`
	LastRollover time.Time `json:"last_rollover"`
	Paused       bool      `json:"paused"`
	Notes        string    `json:"notes"`
}

type WarmupService interface {
	Get(ctx context.Context, userID string) (WarmupState, error)
}

var ErrWarmupNotFound = errors.New("warmup state not found")

func registerWarmupRoutes(r chi.Router, authService AuthService, svc WarmupService) {
	r.Get("/api/warmup", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		state, err := svc.Get(r.Context(), user.ID)
		if err != nil {
			if errors.Is(err, ErrWarmupNotFound) {
				writeJSON(w, http.StatusOK, map[string]any{"warmup": nil})
				return
			}
			writeError(w, http.StatusInternalServerError, "warmup_load_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"warmup": state})
	})
}

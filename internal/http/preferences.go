package apphttp

import (
	"context"
	"net/http"

	"github.com/abhinav-yadav-official/Kleos/internal/preferences"
	"github.com/go-chi/chi/v5"
)

type PreferencesService interface {
	Get(ctx context.Context, userID string) (preferences.Record, error)
	Replace(ctx context.Context, userID string, input preferences.Record) (preferences.Record, error)
}

type preferencesResponse struct {
	Preferences preferences.Record `json:"preferences"`
}

func registerPreferencesRoutes(r chi.Router, authService AuthService, preferencesService PreferencesService) {
	r.Get("/api/preferences", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		record, err := preferencesService.Get(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "preferences_get_failed", "could not load preferences")
			return
		}
		writeJSON(w, http.StatusOK, preferencesResponse{Preferences: record})
	})

	r.Put("/api/preferences", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		var req preferences.Record
		if !decodeJSON(w, r, &req) {
			return
		}
		record, err := preferencesService.Replace(r.Context(), user.ID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "preferences_invalid", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, preferencesResponse{Preferences: record})
	})
}

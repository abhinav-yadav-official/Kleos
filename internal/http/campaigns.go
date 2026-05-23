package apphttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/abhinav-yadav-official/Kleos/internal/campaigns"
	"github.com/go-chi/chi/v5"
)

type CampaignService interface {
	Create(ctx context.Context, userID string, in campaigns.CreateInput) (campaigns.Campaign, error)
	List(ctx context.Context, userID string) ([]campaigns.WithCounts, error)
	Get(ctx context.Context, userID, id string) (campaigns.WithCounts, error)
	SetStatus(ctx context.Context, userID, id, status string) (campaigns.Campaign, error)
	ListMatches(ctx context.Context, userID, campaignID, state string, limit, offset int) ([]campaigns.MatchRow, error)
}

type createCampaignRequest struct {
	Name     string `json:"name"`
	ResumeID string `json:"resume_id"`
	SMTPID   string `json:"smtp_id"`
}

func registerCampaignRoutes(r chi.Router, authService AuthService, svc CampaignService) {
	r.Post("/api/campaigns", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		var req createCampaignRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		c, err := svc.Create(r.Context(), user.ID, campaigns.CreateInput{
			Name: req.Name, ResumeID: req.ResumeID, SMTPID: req.SMTPID,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "campaign_create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"campaign": c})
	})

	r.Get("/api/campaigns", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		list, err := svc.List(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "campaign_list_failed", "could not list campaigns")
			return
		}
		if list == nil {
			list = []campaigns.WithCounts{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"campaigns": list})
	})

	r.Get("/api/campaigns/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		id := chi.URLParam(r, "id")
		c, err := svc.Get(r.Context(), user.ID, id)
		if err != nil {
			if errors.Is(err, campaigns.ErrNotFound) {
				writeError(w, http.StatusNotFound, "campaign_not_found", "campaign not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "campaign_get_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"campaign": c})
	})

	setStatus := func(status string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user, ok := authenticatedUser(w, r, authService)
			if !ok {
				return
			}
			id := chi.URLParam(r, "id")
			c, err := svc.SetStatus(r.Context(), user.ID, id, status)
			if err != nil {
				if errors.Is(err, campaigns.ErrNotFound) {
					writeError(w, http.StatusNotFound, "campaign_not_found", "campaign not found")
					return
				}
				writeError(w, http.StatusBadRequest, "campaign_status_failed", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"campaign": c})
		}
	}
	r.Post("/api/campaigns/{id}/pause", setStatus(campaigns.StatusPaused))
	r.Post("/api/campaigns/{id}/resume", setStatus(campaigns.StatusActive))
	r.Post("/api/campaigns/{id}/archive", setStatus(campaigns.StatusArchived))

	r.Get("/api/campaigns/{id}/matches", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		id := chi.URLParam(r, "id")
		state := r.URL.Query().Get("state")
		limit := parseIntDefault(r.URL.Query().Get("limit"), 50, 1, 500)
		offset := parseIntDefault(r.URL.Query().Get("offset"), 0, 0, 1_000_000)
		rows, err := svc.ListMatches(r.Context(), user.ID, id, state, limit, offset)
		if err != nil {
			if errors.Is(err, campaigns.ErrNotFound) {
				writeError(w, http.StatusNotFound, "campaign_not_found", "campaign not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "matches_list_failed", err.Error())
			return
		}
		if rows == nil {
			rows = []campaigns.MatchRow{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"matches": rows, "limit": limit, "offset": offset})
	})
}

func parseIntDefault(s string, def, lo, hi int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

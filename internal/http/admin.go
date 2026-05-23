package apphttp

import (
	"context"
	"net/http"

	"github.com/abhinav-yadav-official/Kleos/internal/emailfinder"
	"github.com/go-chi/chi/v5"
)

type AdminService interface {
	UpsertCompanyContact(ctx context.Context, slug, careersURL, domain, githubOrg string) (string, error)
	PersistCandidates(ctx context.Context, companyID string, cands []emailfinder.Candidate) (int, error)
	AddToDenylist(ctx context.Context, email, reason string) error
}

type adminRecruitersRequest struct {
	CompanySlug string                `json:"company_slug"`
	Domain      string                `json:"domain"`
	CareersURL  string                `json:"careers_url"`
	GitHubOrg   string                `json:"github_org"`
	Emails      []adminRecruiterEntry `json:"emails"`
}

type adminRecruiterEntry struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

type adminDenylistRequest struct {
	Email  string `json:"email"`
	Reason string `json:"reason"`
}

func registerAdminRoutes(r chi.Router, authService AuthService, svc AdminService) {
	r.Post("/api/admin/recruiters", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireAdmin(w, r, authService)
		if !ok {
			return
		}
		_ = user
		var req adminRecruitersRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.CompanySlug == "" || len(req.Emails) == 0 {
			writeError(w, http.StatusBadRequest, "admin_invalid", "company_slug and emails are required")
			return
		}
		companyID, err := svc.UpsertCompanyContact(r.Context(), req.CompanySlug, req.CareersURL, req.Domain, req.GitHubOrg)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "admin_company_failed", err.Error())
			return
		}
		cands := make([]emailfinder.Candidate, 0, len(req.Emails))
		for _, e := range req.Emails {
			cands = append(cands, emailfinder.Candidate{
				Email:      e.Email,
				Name:       e.Name,
				Title:      e.Title,
				Source:     emailfinder.SourceManual,
				Confidence: emailfinder.ConfidenceHigh,
			})
		}
		n, err := svc.PersistCandidates(r.Context(), companyID, cands)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "admin_persist_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"company_id": companyID,
			"inserted":   n,
			"submitted":  len(req.Emails),
		})
	})

	r.Post("/api/admin/denylist", func(w http.ResponseWriter, r *http.Request) {
		_, ok := requireAdmin(w, r, authService)
		if !ok {
			return
		}
		var req adminDenylistRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if err := svc.AddToDenylist(r.Context(), req.Email, req.Reason); err != nil {
			writeError(w, http.StatusBadRequest, "denylist_invalid", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
}

func requireAdmin(w http.ResponseWriter, r *http.Request, authService AuthService) (authUser, bool) {
	user, ok := authenticatedUser(w, r, authService)
	if !ok {
		return authUser{}, false
	}
	if !user.IsAdmin {
		writeError(w, http.StatusForbidden, "forbidden", "admin only")
		return authUser{}, false
	}
	return user, true
}

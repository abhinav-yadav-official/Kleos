package apphttp

import (
	"net/http"

	"github.com/abhinav-yadav-official/Kleos/internal/emailfinder"
	"github.com/go-chi/chi/v5"
)

// registerRecipientsRoutes mounts the user-scoped recipient paste flow. Unlike
// /api/admin/recruiters this requires no is_admin role; rows still land in the
// shared recruiters table so the email finder picks them up on the next pass.
// Audit logs are stamped with actor="user" so misuse is attributable.
func registerRecipientsRoutes(r chi.Router, authService AuthService, svc AdminService, audit AuditWriter) {
	r.Post("/api/recipients", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		var req adminRecruitersRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.CompanySlug == "" || len(req.Emails) == 0 {
			writeError(w, http.StatusBadRequest, "recipients_invalid", "company_slug and emails are required")
			return
		}
		companyID, err := svc.UpsertCompanyContact(r.Context(), req.CompanySlug, req.CareersURL, req.Domain, req.GitHubOrg)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "recipients_company_failed", err.Error())
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
			writeError(w, http.StatusInternalServerError, "recipients_persist_failed", err.Error())
			return
		}
		audit.Write(r.Context(), user.ID, "user", "user_recipients_added", req.CompanySlug, map[string]any{
			"inserted": n, "submitted": len(req.Emails),
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"company_id": companyID,
			"inserted":   n,
			"submitted":  len(req.Emails),
		})
	})
}

var _ = chi.URLParam // keep chi import even if route file later changes

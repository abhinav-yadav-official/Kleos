package apphttp

import (
	"context"
	"net/http"

	"github.com/abhinav-yadav-official/Kleos/internal/smtpcred"
	"github.com/go-chi/chi/v5"
)

type SMTPService interface {
	Create(ctx context.Context, userID string, input smtpcred.CreateInput) (smtpcred.Credential, error)
	List(ctx context.Context, userID string) ([]smtpcred.Credential, error)
	Verify(ctx context.Context, userID, id string) (smtpcred.VerifyResult, error)
	SetPrimary(ctx context.Context, userID, id string) (smtpcred.Credential, error)
	Delete(ctx context.Context, userID, id string) error
}

func registerSMTPRoutes(r chi.Router, authService AuthService, smtpService SMTPService) {
	r.Get("/api/smtp", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		records, err := smtpService.List(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "smtp_list_failed", "could not list SMTP credentials")
			return
		}
		writeJSON(w, http.StatusOK, map[string][]smtpcred.Credential{"smtp": records})
	})

	r.Post("/api/smtp", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		var req smtpcred.CreateInput
		if !decodeJSON(w, r, &req) {
			return
		}
		record, err := smtpService.Create(r.Context(), user.ID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, "smtp_create_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]smtpcred.Credential{"smtp": record})
	})

	r.Post("/api/smtp/{id}/verify", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		result, err := smtpService.Verify(r.Context(), user.ID, chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "smtp_not_found", "SMTP credential not found")
			return
		}
		writeJSON(w, http.StatusOK, result)
	})

	r.Post("/api/smtp/{id}/primary", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		record, err := smtpService.SetPrimary(r.Context(), user.ID, chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "smtp_not_found", "SMTP credential not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]smtpcred.Credential{"smtp": record})
	})

	r.Delete("/api/smtp/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := authenticatedUser(w, r, authService)
		if !ok {
			return
		}
		if err := smtpService.Delete(r.Context(), user.ID, chi.URLParam(r, "id")); err != nil {
			writeError(w, http.StatusNotFound, "smtp_not_found", "SMTP credential not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
}

func authenticatedUser(w http.ResponseWriter, r *http.Request, authService AuthService) (authUser, bool) {
	token, err := bearerToken(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
		return authUser{}, false
	}
	user, err := authService.UserFromAccessToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
		return authUser{}, false
	}
	return authUser{ID: user.ID}, true
}

type authUser struct {
	ID string
}

package apphttp

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/auth"
	"github.com/go-chi/chi/v5"
)

// GoogleAuthService is the narrow surface registerGoogleAuthRoutes needs from
// the auth.Service.
type GoogleAuthService interface {
	EnsureGoogleUser(ctx context.Context, googleSub, email, name string) (auth.Result, error)
}

const (
	googleOAuthStateCookie = "kleos_google_oauth_state"
	googleStateCookieMaxAge = 600
)

// FrontendURL is where the callback redirects after a successful login. The
// browser receives the access + refresh tokens in the URL fragment; the SPA
// reads window.location.hash on mount, persists them, and replaces the URL.
type googleRouterDeps struct {
	OAuth       *auth.GoogleOAuth
	Auth        GoogleAuthService
	Audit       AuditWriter
	FrontendURL string // e.g. https://abhiyadav.in/kleos
	Secure      bool
}

func registerGoogleAuthRoutes(r chi.Router, deps googleRouterDeps) {
	if deps.OAuth == nil || deps.Auth == nil {
		return
	}
	frontend := strings.TrimRight(deps.FrontendURL, "/")
	audit := deps.Audit
	if audit == nil {
		audit = noopAudit{}
	}

	r.Get("/api/auth/google/start", func(w http.ResponseWriter, r *http.Request) {
		next := safeNext(r.URL.Query().Get("next"))
		state, cookieValue, err := newGoogleOAuthState(next)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "google_state", "could not create state")
			return
		}
		setGoogleStateCookie(w, cookieValue, deps.Secure)
		http.Redirect(w, r, deps.OAuth.AuthCodeURL(state), http.StatusSeeOther)
	})

	r.Get("/api/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
		failRedirect := func(reason string, err error) {
			if err != nil {
				slog.Warn("google callback failed", "reason", reason, "error", err.Error())
			} else {
				slog.Warn("google callback failed", "reason", reason)
			}
			http.Redirect(w, r, frontend+"/?google_error="+url.QueryEscape(reason), http.StatusSeeOther)
		}
		if r.URL.Query().Get("error") != "" {
			failRedirect(r.URL.Query().Get("error"), nil)
			return
		}
		cookie, err := r.Cookie(googleOAuthStateCookie)
		if err != nil {
			failRedirect("missing_state", err)
			return
		}
		clearGoogleStateCookie(w, deps.Secure)
		next, ok := parseGoogleOAuthState(r.URL.Query().Get("state"), cookie.Value)
		if !ok {
			failRedirect("invalid_state", nil)
			return
		}
		gu, err := deps.OAuth.ExchangeUser(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			failRedirect("exchange_failed", err)
			return
		}
		result, err := deps.Auth.EnsureGoogleUser(r.Context(), gu.Sub, gu.Email, gu.Name)
		if err != nil {
			failRedirect("ensure_failed", err)
			return
		}
		audit.Write(r.Context(), result.User.ID, "user", "google_signin", result.User.Email, nil)

		target := frontend + "/"
		if next != "" {
			target = frontend + next
		}
		frag := url.Values{}
		frag.Set("access", result.Access)
		frag.Set("refresh", result.Refresh)
		http.Redirect(w, r, target+"#"+frag.Encode(), http.StatusSeeOther)
	})
}

// safeNext only allows in-app relative paths starting with `/`.
func safeNext(next string) string {
	next = strings.TrimSpace(next)
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") {
		return ""
	}
	return next
}

func newGoogleOAuthState(next string) (state, cookieValue string, err error) {
	state, err = auth.NewRefreshToken() // reuse the 32-byte random helper
	if err != nil {
		return "", "", err
	}
	enc := base64.RawURLEncoding.EncodeToString([]byte(next))
	return state, state + "." + enc, nil
}

func parseGoogleOAuthState(state, cookieValue string) (string, bool) {
	state = strings.TrimSpace(state)
	parts := strings.SplitN(cookieValue, ".", 2)
	if state == "" || len(parts) != 2 {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(state), []byte(parts[0])) != 1 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	return string(raw), true
}

func setGoogleStateCookie(w http.ResponseWriter, value string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookie,
		Value:    value,
		Path:     "/kleos/api/auth/google/",
		Expires:  time.Now().Add(time.Duration(googleStateCookieMaxAge) * time.Second),
		MaxAge:   googleStateCookieMaxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearGoogleStateCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     googleOAuthStateCookie,
		Value:    "",
		Path:     "/kleos/api/auth/google/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// guard against unused import in case future edits drop errors usage.
var _ = errors.New

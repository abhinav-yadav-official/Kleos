package apphttp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/auth"
	"github.com/go-chi/chi/v5"
)

type Pinger interface {
	Ping() error
}

type Dependencies struct {
	DB          Pinger
	Redis       Pinger
	Auth        AuthService
	SMTP        SMTPService
	Resumes     ResumeService
	Preferences PreferencesService
	Campaigns   CampaignService
	Admin       AdminService
	Warmup      WarmupService
	Audit       AuditWriter
	Google      GoogleDeps
}

// GoogleDeps wires the Google OAuth flow when the env credentials are set.
// Leave OAuth nil to disable.
type GoogleDeps struct {
	OAuth       *auth.GoogleOAuth
	Service     GoogleAuthService
	FrontendURL string
	Secure      bool
}

// AuditWriter is the narrow interface satisfied by *audit.Logger.
type AuditWriter interface {
	Write(ctx context.Context, userID, actor, action, target string, meta map[string]any)
}

// Rate limit defaults from §18 security checklist.
const (
	rateLimitUnauthPerMin     = 60
	rateLimitAuthedPerMin     = 600
	rateLimitAuthRoutesPerMin = 5
)

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

	globalLimiter := newRateLimiter(rateLimiterConfig{Limit: rateLimitAuthedPerMin, Window: time.Minute, Capacity: 50_000, Now: time.Now})
	authRouteLimiter := newRateLimiter(rateLimiterConfig{Limit: rateLimitAuthRoutesPerMin, Window: time.Minute, Capacity: 50_000, Now: time.Now})
	_ = rateLimitUnauthPerMin // reserved for unauth-only paths if added later
	r.Use(func(next http.Handler) http.Handler {
		// Apply global per-user-or-IP limit (600/min when authenticated, else per-IP)
		return rateLimitByUser(globalLimiter, deps.Auth)(next)
	})

	r.Get("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	r.Get("/api/readyz", func(w http.ResponseWriter, r *http.Request) {
		if deps.DB == nil || deps.Redis == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "missing_dependency"})
			return
		}
		if err := deps.DB.Ping(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "db_unavailable"})
			return
		}
		if err := deps.Redis.Ping(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "redis_unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	audit := deps.Audit
	if audit == nil {
		audit = noopAudit{}
	}
	if deps.Auth != nil {
		registerAuthRoutes(r, deps.Auth, rateLimitByIP(authRouteLimiter), audit)
	}
	if deps.Auth != nil && deps.SMTP != nil {
		registerSMTPRoutes(r, deps.Auth, deps.SMTP, audit)
	}
	if deps.Auth != nil && deps.Resumes != nil {
		registerResumeRoutes(r, deps.Auth, deps.Resumes)
	}
	if deps.Auth != nil && deps.Preferences != nil {
		registerPreferencesRoutes(r, deps.Auth, deps.Preferences)
	}
	if deps.Auth != nil && deps.Campaigns != nil {
		registerCampaignRoutes(r, deps.Auth, deps.Campaigns)
	}
	if deps.Auth != nil && deps.Admin != nil {
		registerAdminRoutes(r, deps.Auth, deps.Admin, audit)
		registerRecipientsRoutes(r, deps.Auth, deps.Admin, audit)
	}
	if deps.Auth != nil && deps.Warmup != nil {
		registerWarmupRoutes(r, deps.Auth, deps.Warmup)
	}
	if deps.Google.OAuth != nil && deps.Google.Service != nil {
		registerGoogleAuthRoutes(r, googleRouterDeps{
			OAuth:       deps.Google.OAuth,
			Auth:        deps.Google.Service,
			Audit:       audit,
			FrontendURL: deps.Google.FrontendURL,
			Secure:      deps.Google.Secure,
		})
	}

	return r
}

type noopAudit struct{}

func (noopAudit) Write(context.Context, string, string, string, string, map[string]any) {}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

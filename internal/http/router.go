package apphttp

import (
	"encoding/json"
	"net/http"

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
}

func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()

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

	if deps.Auth != nil {
		registerAuthRoutes(r, deps.Auth)
	}
	if deps.Auth != nil && deps.SMTP != nil {
		registerSMTPRoutes(r, deps.Auth, deps.SMTP)
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
		registerAdminRoutes(r, deps.Auth, deps.Admin)
	}
	if deps.Auth != nil && deps.Warmup != nil {
		registerWarmupRoutes(r, deps.Auth, deps.Warmup)
	}

	return r
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

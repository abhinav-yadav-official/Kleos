package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/audit"
	"github.com/abhinav-yadav-official/Kleos/internal/auth"
	"github.com/abhinav-yadav-official/Kleos/internal/campaigns"
	"github.com/abhinav-yadav-official/Kleos/internal/config"
	appcrypto "github.com/abhinav-yadav-official/Kleos/internal/crypto"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
	"github.com/abhinav-yadav-official/Kleos/internal/emailfinder"
	apphttp "github.com/abhinav-yadav-official/Kleos/internal/http"
	"github.com/abhinav-yadav-official/Kleos/internal/preferences"
	"github.com/abhinav-yadav-official/Kleos/internal/resume"
	"github.com/abhinav-yadav-official/Kleos/internal/smtpcred"
)

func main() {
	cfg := config.Load()

	postgres, err := db.ConnectPostgres(context.Background(), cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer postgres.Close()

	redisClient := db.ConnectRedis(cfg.RedisAddr, cfg.RedisPassword)
	defer func() {
		if err := redisClient.Close(); err != nil {
			slog.Error("close redis", "error", err)
		}
	}()

	accessTTL, err := time.ParseDuration(cfg.JWTAccessTTL)
	if err != nil {
		slog.Error("parse JWT_ACCESS_TTL", "error", err)
		os.Exit(1)
	}
	refreshTTL, err := time.ParseDuration(cfg.JWTRefreshTTL)
	if err != nil {
		slog.Error("parse JWT_REFRESH_TTL", "error", err)
		os.Exit(1)
	}
	authService := auth.NewServiceWithRotation(postgres.Pool(), cfg.JWTSecret, cfg.JWTSecretPrevious, accessTTL, refreshTTL)
	smtpCodec, err := appcrypto.NewAESGCM(cfg.SMTPKey)
	if err != nil {
		slog.Error("create SMTP credential codec", "error", err)
		os.Exit(1)
	}
	smtpService := smtpcred.NewService(postgres.Pool(), smtpCodec)
	resumeService := resume.NewService(postgres.Pool(), cfg.ResumeStorage)
	preferencesService := preferences.NewService(postgres.Pool())
	campaignService := campaigns.NewService(postgres.Pool())
	adminService := emailfinder.NewService(postgres.Pool(), nil, nil)
	warmupAdapter := newWarmupAdapter(postgres.Pool())
	auditLogger := audit.NewLogger(postgres.Pool())

	googleOAuth, err := auth.GoogleOAuthFromEnv(cfg.AppBaseURL)
	if err != nil {
		slog.Error("google oauth init", "error", err)
		os.Exit(1)
	}
	if googleOAuth == nil {
		slog.Info("google oauth disabled (GOOGLE_CLIENT_ID/GOOGLE_CLIENT_SECRET not set)")
	} else {
		slog.Info("google oauth enabled", "redirect_url", googleOAuth.RedirectURL)
	}
	frontendBase := strings.TrimRight(cfg.AppBaseURL, "/")
	if !strings.HasSuffix(frontendBase, "/kleos") {
		frontendBase += "/kleos"
	}
	googleDeps := apphttp.GoogleDeps{
		OAuth:       googleOAuth,
		Service:     authService,
		FrontendURL: frontendBase,
		Secure:      strings.HasPrefix(cfg.AppBaseURL, "https://"),
	}

	server := &http.Server{
		Addr: ":" + cfg.AppPort,
		Handler: apphttp.NewRouter(apphttp.Dependencies{
			DB: postgres, Redis: redisClient, Auth: authService, SMTP: smtpService,
			Resumes: resumeService, Preferences: preferencesService, Campaigns: campaignService,
			Admin: adminService, Warmup: warmupAdapter, Audit: auditLogger, Google: googleDeps,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errs := make(chan error, 1)
	go func() {
		slog.Info("api listening", "addr", server.Addr)
		errs <- server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signals:
		slog.Info("shutdown signal received", "signal", sig.String())
	case err := <-errs:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("server shutdown failed", "error", err)
		os.Exit(1)
	}
}

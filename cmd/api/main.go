package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	"github.com/abhinav-yadav-official/Kleos/internal/db"
	apphttp "github.com/abhinav-yadav-official/Kleos/internal/http"
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

	server := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           apphttp.NewRouter(apphttp.Dependencies{DB: postgres, Redis: redisClient}),
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

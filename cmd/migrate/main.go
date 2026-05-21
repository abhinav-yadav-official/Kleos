package main

import (
	"database/sql"
	"log/slog"
	"os"

	"github.com/abhinav-yadav-official/Kleos/internal/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	cfg := config.Load()
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	args := []string{}
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}

	db, err := sql.Open("pgx", cfg.DBDSN)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		slog.Error("set goose dialect", "error", err)
		os.Exit(1)
	}
	if err := goose.Run(command, db, "migrations", args...); err != nil {
		slog.Error("run migrations", "command", command, "error", err)
		os.Exit(1)
	}
}

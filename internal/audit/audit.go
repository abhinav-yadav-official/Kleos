package audit

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger struct {
	pool *pgxpool.Pool
}

func NewLogger(pool *pgxpool.Pool) *Logger {
	return &Logger{pool: pool}
}

// Write inserts a row into audit_log. Failures are logged but never propagated;
// audit must not block business logic.
func (l *Logger) Write(ctx context.Context, userID, actor, action, target string, meta map[string]any) {
	if l == nil || l.pool == nil {
		return
	}
	metaJSON := []byte("{}")
	if len(meta) > 0 {
		if b, err := json.Marshal(meta); err == nil {
			metaJSON = b
		}
	}
	var userArg any
	if userID != "" {
		userArg = userID
	} else {
		userArg = nil
	}
	if _, err := l.pool.Exec(ctx, `
		INSERT INTO audit_log (user_id, actor, action, target, meta)
		VALUES ($1::uuid, $2, $3, $4, $5::jsonb)
	`, userArg, actor, action, target, string(metaJSON)); err != nil {
		slog.Warn("audit write failed", "action", action, "error", err)
	}
}

package smtpcred

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestEnsureWarmupStateCreatesDayOneRowIdempotently(t *testing.T) {
	exec := &recordingExec{}

	err := ensureWarmupState(context.Background(), exec, "user-1", "smtp-1", 5)
	if err != nil {
		t.Fatalf("ensure warmup state: %v", err)
	}

	if !strings.Contains(exec.sql, "INSERT INTO warmup_state") {
		t.Fatalf("query should insert warmup_state row: %s", exec.sql)
	}
	if !strings.Contains(exec.sql, "ON CONFLICT (user_id) DO NOTHING") {
		t.Fatalf("query should be idempotent for existing user warmup state: %s", exec.sql)
	}
	if got, want := exec.args, []any{"user-1", "smtp-1", 5}; len(got) != len(want) {
		t.Fatalf("args len = %d, want %d", len(got), len(want))
	} else {
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("arg %d = %#v, want %#v", i, got[i], want[i])
			}
		}
	}
}

type recordingExec struct {
	sql  string
	args []any
}

func (e *recordingExec) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	e.sql = sql
	e.args = args
	return pgconn.CommandTag{}, nil
}

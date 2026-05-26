package audit

import (
	"context"
	"testing"
)

// Logger with nil pool is a no-op safety net (used in tests where we don't
// stand up a database).
func TestNilLoggerSafe(t *testing.T) {
	var l *Logger
	l.Write(context.Background(), "", "system", "no_op", "", nil) // must not panic
}

func TestZeroLoggerSafe(t *testing.T) {
	l := &Logger{}
	l.Write(context.Background(), "", "system", "no_op", "", nil)
}

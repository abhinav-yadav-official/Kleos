package contentgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Generator turns a rendered prompt into a Result. CLICodex shells out to the
// real Codex CLI; tests use a fake.
type Generator interface {
	Generate(ctx context.Context, prompt string) (Result, error)
}

// CLICodex invokes the Codex CLI as a subprocess. `Cmd` is the executable
// (default "codex"); `Args` defaults to `["exec", "--json", "--no-stream"]`.
type CLICodex struct {
	Cmd     string
	Args    []string
	Timeout time.Duration
}

func NewCLICodex(cmd string, timeout time.Duration) *CLICodex {
	if cmd == "" {
		cmd = "codex"
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &CLICodex{
		Cmd:     cmd,
		Args:    []string{"exec", "--json", "--no-stream"},
		Timeout: timeout,
	}
}

var ErrEmptyOutput = errors.New("codex returned empty output")

func (c *CLICodex) Generate(ctx context.Context, prompt string) (Result, error) {
	runCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, c.Cmd, c.Args...)
	cmd.Stdin = strings.NewReader(prompt)
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("codex run: %w (stderr: %s)", err, truncate(errBuf.String(), 200))
	}
	raw := bytes.TrimSpace(out.Bytes())
	if len(raw) == 0 {
		return Result{}, ErrEmptyOutput
	}
	return ParseGeneratorOutput(raw)
}

// ParseGeneratorOutput extracts the {"variants": [...]} payload from raw
// Codex output. It tolerates a leading text/JSON-Lines preamble by scanning
// for the first '{' that starts a balanced JSON object containing "variants".
func ParseGeneratorOutput(raw []byte) (Result, error) {
	// Try direct decode first.
	var r Result
	if err := json.Unmarshal(raw, &r); err == nil && len(r.Variants) > 0 {
		return r, nil
	}
	// Scan for first balanced JSON object containing "variants".
	for i := 0; i < len(raw); i++ {
		if raw[i] != '{' {
			continue
		}
		end := findMatchingBrace(raw, i)
		if end < 0 {
			break
		}
		chunk := raw[i : end+1]
		if !bytes.Contains(chunk, []byte("\"variants\"")) {
			continue
		}
		if err := json.Unmarshal(chunk, &r); err == nil && len(r.Variants) > 0 {
			return r, nil
		}
	}
	return Result{}, fmt.Errorf("could not parse variants from codex output: %s", truncate(string(raw), 200))
}

func findMatchingBrace(b []byte, start int) int {
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(b); i++ {
		c := b[i]
		if escape {
			escape = false
			continue
		}
		if inStr {
			switch c {
			case '\\':
				escape = true
			case '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

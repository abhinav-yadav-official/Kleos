# AGENTS.md

Guidance for Codex and other coding agents working in this repository.

## Project Purpose

Kleos is a privacy-first job-outreach SaaS. Users connect their own SMTP sender, upload a resume, set search preferences, and Kleos discovers roles, finds recruiter emails, drafts tailored outreach, and sends messages gradually with deliverability protection.

The backend is Go 1.23. The frontend is a statically exported Next.js app served under the `/kleos` base path.

## Directory Structure

- `cmd/api` - HTTP API entrypoint.
- `cmd/migrate` - Goose migration runner.
- `cmd/worker-*` - one-shot pipeline workers for scraping, campaign ticks, email finding, content generation, sending, warmup rollover, and recipient prefetch.
- `internal/http` - chi router, route handlers, request/response handling, rate limits, and test fakes.
- `internal/auth`, `internal/smtpcred`, `internal/resume`, `internal/preferences`, `internal/campaigns`, `internal/contentgen`, `internal/sender`, `internal/scraper`, `internal/emailfinder`, `internal/audit`, `internal/crypto` - domain packages.
- `internal/db` - Postgres and Redis connection helpers.
- `internal/config` - env-based configuration with local defaults.
- `migrations` - Goose SQL migrations.
- `web` - Next.js 14 frontend, Tailwind CSS, static export to `web/out`.
- `deploy` - Docker Compose, runtime Dockerfile, Nginx location config, deploy script.
- `scripts` - backup/restore and sender test helpers.
- `seeds` - seed data for shared recipient/company pools.
- `docs` - screenshots and supporting project docs.

## Build, Run, Test

Backend:

```sh
make build        # build all Go binaries into bin/
make test         # go test ./... -count=1
make lint         # go vet ./...
make up           # start local Postgres on 5433 and Redis on 6380
make down         # stop local dev services
make migrate      # go run ./cmd/migrate up
make run-api      # go run ./cmd/api
```

Useful targeted test form:

```sh
go test ./internal/campaigns -run TestTick -count=1
```

Frontend:

```sh
cd web
npm ci
npm run dev
npm run build     # next build; static export goes to web/out
```

CI runs `go vet`, `golangci-lint` v1.60, `go test ./... -race -count=1`, Go builds for all commands, and `npm run build` under Node 24.

## Configuration

Copy `.env.example` to `.env` for local work. Configuration is read from environment variables in `internal/config`; the API does not use CLI flags. Local defaults expect:

- Postgres: `postgres://kleos:kleos@localhost:5433/kleos?sslmode=disable`
- Redis: `localhost:6380`
- API port: `8080`
- app base URL: `http://localhost:8080`

SMTP credentials are encrypted with AES-GCM using `SMTP_CRED_ENCRYPTION_KEY`, a 32-byte hex key. Production serves behind Nginx at `/kleos`; keep frontend/API paths compatible with that subpath.

## Architecture Notes

- `cmd/*` packages should stay thin: load config, connect dependencies, wire services, run.
- `internal/http.NewRouter` accepts service interfaces via dependencies. Routes are registered only when their dependency exists, so optional features can degrade cleanly.
- `cmd/api` serves HTTP only. Pipeline state advances through worker binaries.
- The outreach pipeline is a state machine over `campaign_matches.state`; workers claim rows with guarded updates to stay safe under concurrency.
- Postgres is the source of truth. Redis is used for rate limiting and ephemeral coordination.
- Content generation shells out to the Codex CLI and then runs spam scoring to select the safest variant.
- Migrations use Goose SQL annotations (`-- +goose Up` / `-- +goose Down`).

## Conventions And Gotchas

- Prefer small, package-local changes that match the existing service/interface style.
- Keep tests colocated as `*_test.go`; HTTP tests commonly use `*_fake_test.go` service fakes.
- Use `log/slog` for structured logging.
- Do not hardcode secrets. Use env config and `.env.example` for documented variables.
- Preserve `/kleos` behavior: `web/next.config.js` sets `basePath: "/kleos"`, and `web/lib/api.ts` calls `/kleos/api`.
- Google OAuth redirect URI must match the deployed subpath: `{APP_BASE_URL}/kleos/api/auth/google/callback`.
- Read `plan.md` when section references like `§11` appear in code; it is the long-form design source. `checkpoints.txt` is a build journal, not implementation guidance.
- Avoid advancing pipeline states in the API. Add or adjust worker behavior in the relevant `cmd/worker-*` and `internal/*` package.

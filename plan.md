# Kleos — plan.md

Agent-facing build plan for the Kleos project. Read top to bottom before starting. Every phase has acceptance criteria. Do not skip phases. Do not invent features not listed here.

Execution checkpoint rule: after every completed implementation step, append a short entry to `checkpoints.txt` with timestamp, phase/task, files changed, commands run, verification result, and any unresolved decision. Before resuming after a context switch, read `checkpoints.txt` first to recover old context, then reopen the relevant section of this plan.

---

## 0. Locked decisions (do not relitigate)

| Decision | Value |
|---|---|
| Project | Kleos |
| Git repo name | `Kleos` |
| Tenancy | Public signup SaaS |
| SMTP | User-supplied only (Gmail app password, SES, custom) |
| Job sources v1 | Greenhouse, Lever, Ashby, RemoteOK, Wellfound, Indeed, Naukri, LinkedIn (best-effort, may break) |
| Flow | Fully automatic: scrape → find email → generate → send |
| Recruiter email sources | GitHub commit mining + careers page mailto scraping + manual admin paste |
| Content gen | Codex CLI with subscription auth, subprocess invoked from Go |
| Sending domain | User brings own warmed domain. App must not provide one. |
| Tracking | None. No pixels, no link wrapping. Privacy-first. |
| Follow-ups | Out of v1. Schema must allow adding later without migration pain. |
| Topology | Single VPS, docker compose for everything (api, workers, postgres, redis) |
| CI/CD | GitHub Actions → SSH rsync of built artifacts → docker compose up |
| Monitoring | Structured JSON logs to file, logrotate. No Sentry/Prom in v1. |
| VPS | `abhiyadav.in`, Ubuntu 24.04.3 LTS, 4 GB RAM, 2 vCPU, 48 GB disk |
| SSH target | local alias `vps` = `abhinav@abhiyadav.in:2022`; CI should use `deploy@abhiyadav.in:2022` after first-time setup |
| Tone customization | Presets (formal/casual/technical/warm) + optional custom addendum |
| Resume format | PDF only, parsed via pdftotext |
| Public endpoint | `https://abhiyadav.in/kleos/` (frontend) and `https://abhiyadav.in/kleos/api/` (backend) |

### VPS facts verified on 2026-05-21

- `/opt/kleos` does not exist yet. First-time setup must create it.
- `deploy` user does not exist yet. First-time setup must create it and add it to the `docker` group.
- Docker is installed (`Docker 29.5.1`, Compose `v5.1.3`). Current `abhinav` login is not in the Docker group, so manual inspection uses `sudo docker ...`.
- Existing nginx config for `abhiyadav.in` includes `/etc/nginx/snippets/site-locations.conf`; add a Kleos snippet there rather than creating another server block.
- Host ports already in use: `5432` (Postgres), `6379` (Redis), `8082`, `8085`, `8086`, `3000`, `7001`. Port `8080` is free and reserved for Kleos API.

---

## 1. Stack

- **Backend:** Go 1.23, `chi` router, `sqlc` for typed SQL, `goose` for migrations, `asynq` for Redis-backed job queue
- **DB:** PostgreSQL 16 (docker)
- **Queue/cache:** Redis 7 (docker)
- **Frontend:** Next.js 14 App Router, static export (`output: 'export'`), Tailwind, shadcn/ui, TanStack Query
- **Content gen:** Codex CLI subprocess, subscription-authenticated on the VPS user's shell
- **PDF parsing:** `pdftotext` (poppler-utils) shell call
- **SMTP client:** `net/smtp` + `github.com/wneessen/go-mail` for richer features
- **Auth:** JWT (HS256), refresh token rotation, bcrypt for password hashing
- **Crypto:** AES-256-GCM for SMTP creds at rest, key in env
- **Reverse proxy:** existing nginx on VPS, new Kleos location snippet included from `/etc/nginx/snippets/site-locations.conf`
- **Container runtime:** Docker + `docker compose` CLI (`Docker 29.5.1`, Compose `v5.1.3` verified on VPS)

### Why these (one-liners)

- `chi` not gin: stdlib-shaped, smaller surface
- `sqlc` not gorm: no ORM tax, generated typed Go from SQL
- `asynq` not custom Redis queue: built-in retries, scheduling, dashboard, dead-letter
- Static Next.js export: nginx serves files, no Node runtime in prod
- All Docker: matches the locked topology, simpler ops than mixed bare-metal

---

## 2. Repository layout

```
kleos/
├── README.md
├── plan.md                              (this file)
├── checkpoints.txt                      step log; append after every completed step
├── Makefile
├── .env.example
├── .gitignore
├── .golangci.yml
├── go.mod
├── go.sum
├── cmd/
│   ├── api/main.go                      HTTP API
│   ├── worker-jobscraper/main.go        scrapes job boards
│   ├── worker-emailfinder/main.go       finds recruiter emails
│   ├── worker-contentgen/main.go        invokes Codex CLI
│   ├── worker-sender/main.go            sends via user SMTP
│   └── migrate/main.go                  goose CLI wrapper
├── internal/
│   ├── auth/                            JWT, bcrypt, middleware
│   ├── config/                          env loading
│   ├── crypto/                          AES-GCM for SMTP creds
│   ├── db/                              sqlc-generated + connection
│   ├── http/                            handlers, middleware, router
│   ├── logger/                          slog setup, JSON to file
│   ├── models/                          shared structs
│   ├── queue/                           asynq client+server wrappers
│   ├── ratelimit/                       per-user send limits
│   ├── resume/                          pdftotext wrapper, parsing
│   ├── scraper/                         interface + impls
│   │   ├── greenhouse.go
│   │   ├── lever.go
│   │   ├── ashby.go
│   │   ├── remoteok.go
│   │   ├── wellfound.go
│   │   ├── indeed.go
│   │   ├── naukri.go
│   │   └── linkedin.go                  best-effort, document fragility
│   ├── emailfinder/                     github + mailto + manual
│   │   ├── github.go
│   │   ├── mailto.go
│   │   └── manual.go
│   ├── contentgen/                      Codex CLI subprocess + prompt
│   │   ├── codex.go
│   │   └── prompt.go                    SEE §10 for prompt rules
│   ├── sender/                          SMTP send + warm-up logic
│   │   ├── smtp.go
│   │   └── warmup.go                    SEE §11
│   └── validate/                        input validation helpers
├── migrations/
│   ├── 001_init.sql
│   ├── 002_jobs_and_recruiters.sql
│   ├── 003_campaigns_and_outbox.sql
│   └── 004_warmup_and_audit.sql
├── sql/queries/                         sqlc input
│   ├── users.sql
│   ├── smtp_credentials.sql
│   ├── campaigns.sql
│   ├── jobs.sql
│   ├── recruiters.sql
│   ├── email_drafts.sql
│   ├── sent_emails.sql
│   └── warmup.sql
├── sqlc.yaml
├── web/                                 Next.js
│   ├── app/
│   │   ├── layout.tsx
│   │   ├── page.tsx                     landing/login
│   │   ├── dashboard/page.tsx
│   │   ├── onboarding/
│   │   │   ├── smtp/page.tsx
│   │   │   ├── resume/page.tsx
│   │   │   └── preferences/page.tsx
│   │   ├── campaigns/
│   │   │   ├── page.tsx                 list
│   │   │   └── [id]/page.tsx            detail
│   │   └── settings/page.tsx
│   ├── components/                      shadcn + custom
│   ├── lib/api.ts                       typed API client
│   ├── lib/auth.ts
│   ├── next.config.js                   output: 'export', basePath: '/kleos'
│   ├── tailwind.config.ts
│   ├── package.json
│   └── tsconfig.json
├── deploy/
│   ├── docker-compose.yml               prod compose
│   ├── docker-compose.dev.yml           dev (only db+redis)
│   ├── Dockerfile.api
│   ├── Dockerfile.worker                multi-binary, ARG WORKER
│   ├── nginx.location.conf              snippet to include
│   ├── deploy.sh                        runs on VPS via ssh
│   └── logrotate.conf
├── scripts/
│   └── backup.sh                        pg_dump backup + 14-day retention
└── .github/
    └── workflows/
        ├── ci.yml                       lint + test on PR
        └── deploy.yml                   deploy on main push
```

---

## 3. Environment variables

`.env.example` (commit this, do not commit real `.env`):

```
# Server
APP_ENV=production
APP_PORT=8080
APP_BASE_URL=https://abhiyadav.in/kleos
LOG_DIR=/var/log/kleos
LOG_LEVEL=info

# DB
DB_DSN=postgres://kleos:CHANGE_ME@postgres:5432/kleos?sslmode=disable

# Redis
REDIS_ADDR=redis:6379
REDIS_PASSWORD=

# Auth
JWT_SECRET=CHANGE_ME_64_BYTES_HEX
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=720h

# Crypto
SMTP_CRED_ENCRYPTION_KEY=CHANGE_ME_32_BYTES_HEX

# Codex
CODEX_BIN=/usr/local/bin/codex
CODEX_TIMEOUT=60s
CODEX_HOME=/home/kleos/.codex          # subscription auth state

# Scraping
GITHUB_TOKEN=CHANGE_ME                       # for email finder + rate limit headroom
SCRAPER_USER_AGENT=kleos/0.1 (+https://abhiyadav.in/kleos)
SCRAPER_HTTP_TIMEOUT=20s

# Send limits (defaults; per-user override in DB)
SEND_RATE_PER_HOUR=20
SEND_RATE_PER_DAY=100
SEND_JITTER_MIN_SECONDS=30
SEND_JITTER_MAX_SECONDS=180

# Warmup defaults (Day N = ceil(BASE * GROWTH^(N-1)), capped)
WARMUP_DAY1_LIMIT=5
WARMUP_GROWTH=1.4
WARMUP_CAP=40
WARMUP_DAYS=21
```

Key generation commands documented in README:
```
openssl rand -hex 64   # JWT_SECRET
openssl rand -hex 32   # SMTP_CRED_ENCRYPTION_KEY
```

---

## 4. Database schema

Single source of truth: `migrations/`. Run via `goose`.

### 4.1 Migration 001 — init

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email         CITEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  name          TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  is_active     BOOLEAN NOT NULL DEFAULT true,
  is_admin      BOOLEAN NOT NULL DEFAULT false,
  daily_send_cap INT NOT NULL DEFAULT 100,
  hourly_send_cap INT NOT NULL DEFAULT 20
);

CREATE TABLE refresh_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,           -- sha256(token)
  expires_at  TIMESTAMPTZ NOT NULL,
  revoked_at  TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

CREATE TABLE smtp_credentials (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label           TEXT NOT NULL,              -- "Gmail personal"
  host            TEXT NOT NULL,
  port            INT NOT NULL,
  username        TEXT NOT NULL,
  password_cipher BYTEA NOT NULL,             -- AES-256-GCM
  password_nonce  BYTEA NOT NULL,
  from_email      CITEXT NOT NULL,
  from_name       TEXT,
  use_tls         BOOLEAN NOT NULL DEFAULT true,
  verified_at     TIMESTAMPTZ,
  last_error      TEXT,
  is_primary      BOOLEAN NOT NULL DEFAULT false,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_smtp_one_primary_per_user
  ON smtp_credentials(user_id) WHERE is_primary;

CREATE TABLE resumes (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  filename    TEXT NOT NULL,
  storage_path TEXT NOT NULL,                 -- /opt/kleos/data/resumes/<uid>/<id>.pdf
  parsed_text TEXT NOT NULL,
  is_active   BOOLEAN NOT NULL DEFAULT true,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_resumes_user_active ON resumes(user_id) WHERE is_active;

CREATE TABLE preferences (
  user_id          UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  job_titles       TEXT[] NOT NULL DEFAULT '{}',
  job_functions    TEXT[] NOT NULL DEFAULT '{}',
  experience_level TEXT NOT NULL DEFAULT 'mid',   -- entry|mid|senior|staff|principal
  locations        TEXT[] NOT NULL DEFAULT '{}',
  keywords_include TEXT[] NOT NULL DEFAULT '{}',
  keywords_exclude TEXT[] NOT NULL DEFAULT '{}',
  remote_only      BOOLEAN NOT NULL DEFAULT false,
  tone_preset      TEXT NOT NULL DEFAULT 'warm',  -- formal|casual|technical|warm
  tone_addendum    TEXT NOT NULL DEFAULT '',      -- user-supplied custom note for the prompt
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.2 Migration 002 — jobs and recruiters

```sql
CREATE TABLE companies (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name          TEXT NOT NULL,
  slug          TEXT UNIQUE NOT NULL,         -- normalized lowercase
  domain        TEXT,
  careers_url   TEXT,
  github_org    TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_companies_domain ON companies(domain);

CREATE TABLE jobs (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source         TEXT NOT NULL,               -- greenhouse|lever|ashby|remoteok|wellfound|indeed|naukri|linkedin
  external_id    TEXT NOT NULL,
  company_id     UUID REFERENCES companies(id),
  title          TEXT NOT NULL,
  description    TEXT NOT NULL,
  location       TEXT,
  remote         BOOLEAN NOT NULL DEFAULT false,
  url            TEXT NOT NULL,
  posted_at      TIMESTAMPTZ,
  scraped_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  raw            JSONB NOT NULL,
  UNIQUE (source, external_id)
);
CREATE INDEX idx_jobs_company ON jobs(company_id);
CREATE INDEX idx_jobs_scraped_at ON jobs(scraped_at DESC);
CREATE INDEX idx_jobs_title_trgm ON jobs USING gin (title gin_trgm_ops);
-- enable pg_trgm
-- CREATE EXTENSION IF NOT EXISTS pg_trgm; (add at top of this migration)

CREATE TABLE recruiters (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id    UUID REFERENCES companies(id),
  email         CITEXT NOT NULL,
  name          TEXT,
  title         TEXT,
  source        TEXT NOT NULL,                -- github|mailto|manual
  confidence    TEXT NOT NULL,                -- high|medium|low
  evidence_url  TEXT,
  is_blocked    BOOLEAN NOT NULL DEFAULT false,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (email, company_id)
);
CREATE INDEX idx_recruiters_company ON recruiters(company_id);

-- Global denylist for emails we must never send to
CREATE TABLE email_denylist (
  email      CITEXT PRIMARY KEY,
  reason     TEXT,
  added_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 4.3 Migration 003 — campaigns and outbox

```sql
CREATE TABLE campaigns (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  status       TEXT NOT NULL DEFAULT 'active',   -- active|paused|archived
  resume_id    UUID NOT NULL REFERENCES resumes(id),
  smtp_id      UUID NOT NULL REFERENCES smtp_credentials(id),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_campaigns_user ON campaigns(user_id);

CREATE TABLE campaign_matches (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  campaign_id  UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
  job_id       UUID NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
  match_score  REAL NOT NULL,
  matched_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  state        TEXT NOT NULL DEFAULT 'new',
    -- new|finding_email|email_found|email_missing|generating|generated|queued|sent|failed|skipped
  UNIQUE (campaign_id, job_id)
);
CREATE INDEX idx_matches_state ON campaign_matches(state);

CREATE TABLE email_drafts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  match_id        UUID NOT NULL REFERENCES campaign_matches(id) ON DELETE CASCADE,
  recruiter_id    UUID NOT NULL REFERENCES recruiters(id),
  variant         INT NOT NULL,                -- 1|2|3
  subject         TEXT NOT NULL,
  body_text       TEXT NOT NULL,
  body_html       TEXT,
  chosen          BOOLEAN NOT NULL DEFAULT false,
  spam_score      REAL,                        -- self-check, see §10
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_drafts_match ON email_drafts(match_id);

CREATE TABLE sent_emails (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id),
  match_id        UUID NOT NULL REFERENCES campaign_matches(id),
  draft_id        UUID NOT NULL REFERENCES email_drafts(id),
  recruiter_email CITEXT NOT NULL,
  smtp_id         UUID NOT NULL REFERENCES smtp_credentials(id),
  message_id      TEXT NOT NULL,               -- RFC 5322 Message-ID
  status          TEXT NOT NULL,               -- sent|bounced|smtp_error|permanent_fail
  smtp_response   TEXT,
  sent_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sent_user_time ON sent_emails(user_id, sent_at DESC);
CREATE INDEX idx_sent_recruiter ON sent_emails(recruiter_email);

-- Once-per-recruiter guard for v1 (no follow-ups)
CREATE UNIQUE INDEX uniq_sent_per_user_per_recruiter
  ON sent_emails(user_id, recruiter_email);
```

### 4.4 Migration 004 — warmup and audit

```sql
CREATE TABLE warmup_state (
  user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  smtp_id         UUID NOT NULL REFERENCES smtp_credentials(id) ON DELETE CASCADE,
  start_date      DATE NOT NULL,
  current_day     INT NOT NULL DEFAULT 1,
  todays_sent     INT NOT NULL DEFAULT 0,
  todays_limit    INT NOT NULL,
  last_rollover   DATE NOT NULL DEFAULT CURRENT_DATE,
  paused          BOOLEAN NOT NULL DEFAULT false,
  notes           TEXT
);

CREATE TABLE audit_log (
  id         BIGSERIAL PRIMARY KEY,
  user_id    UUID REFERENCES users(id),
  actor      TEXT NOT NULL,                    -- user|system|admin
  action     TEXT NOT NULL,
  target     TEXT,
  meta       JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_user_time ON audit_log(user_id, created_at DESC);
```

---

## 5. HTTP API

All routes under `/api/`. All authenticated routes require `Authorization: Bearer <access_token>`.
Content-type: `application/json` everywhere.
Errors: `{ "error": { "code": "string", "message": "string", "details": {...} } }` with appropriate HTTP status.

### Auth
- `POST /api/auth/signup` — `{ email, password, name }` → `{ user, access, refresh }`
- `POST /api/auth/login` — `{ email, password }` → `{ user, access, refresh }`
- `POST /api/auth/refresh` — `{ refresh }` → `{ access, refresh }` (rotation)
- `POST /api/auth/logout` — revokes refresh token
- `GET  /api/auth/me` → current user

### SMTP
- `GET    /api/smtp` — list
- `POST   /api/smtp` — `{ label, host, port, username, password, from_email, from_name, use_tls }` → record (without password)
- `POST   /api/smtp/{id}/verify` — opens connection, NOOP, returns `{ ok, detail }`
- `POST   /api/smtp/{id}/primary` — mark primary
- `DELETE /api/smtp/{id}`

### Resume
- `POST   /api/resumes` — multipart upload, returns parsed text preview
- `GET    /api/resumes` — list
- `POST   /api/resumes/{id}/activate`
- `DELETE /api/resumes/{id}`

### Preferences
- `GET /api/preferences`
- `PUT /api/preferences` — full replace

### Campaigns
- `POST /api/campaigns` — `{ name, resume_id, smtp_id }`
- `GET  /api/campaigns` — list with counts (matches by state)
- `GET  /api/campaigns/{id}` — detail
- `POST /api/campaigns/{id}/pause`
- `POST /api/campaigns/{id}/resume`
- `POST /api/campaigns/{id}/archive`

### Matches / drafts / sent (read-only from UI in v1)
- `GET /api/campaigns/{id}/matches?state=...&limit=...&cursor=...`
- `GET /api/matches/{id}/drafts`
- `GET /api/sent?campaign_id=...&limit=...&cursor=...`

### Admin (is_admin only)
- `POST /api/admin/recruiters` — bulk paste `{ company_slug, emails: [{email, name?, title?}] }`
- `POST /api/admin/denylist` — `{ email, reason }`
- `GET  /api/admin/stats` — system counts

### Health
- `GET /api/healthz` — liveness
- `GET /api/readyz` — checks DB + Redis

---

## 6. Queue jobs (asynq)

Queues defined: `critical`, `default`, `low`. Retry policies inline.

| Task | Queue | Payload | Retries | Backoff |
|---|---|---|---|---|
| `job:scrape:source` | low | `{source, user_id?, since}` | 3 | exp 1m..1h |
| `email:find` | default | `{job_id, campaign_id}` | 5 | exp 30s..30m |
| `content:generate` | default | `{match_id, recruiter_id}` | 3 | exp 1m..15m |
| `email:send` | critical | `{match_id, draft_id}` | 4 | exp 30s..1h |
| `warmup:rollover` | low | none, cron daily 00:05 user-tz UTC fallback | 3 | linear 1m |
| `campaign:tick` | default | `{campaign_id}`, cron every 15m | 1 | none |

`campaign:tick` is the orchestrator: walks active campaigns, finds new matches, enqueues downstream tasks per state machine in §7.

---

## 7. State machine: `campaign_matches.state`

```
                  match created
                       │
                       ▼
                    [new]
                       │ campaign:tick picks it up, enqueue email:find
                       ▼
              [finding_email]
                  │           │
       found       │           │  not found after retries
                  ▼           ▼
          [email_found]   [email_missing] ─── terminal until manual paste
                  │
                  │ campaign:tick enqueues content:generate
                  ▼
              [generating]
                  │
                  ▼
              [generated]
                  │ pick best variant by spam_score, enqueue email:send
                  ▼
                [queued]
                  │           │
       success    │           │  permanent fail
                  ▼           ▼
                [sent]      [failed]
```

`skipped` set when: recruiter on denylist, already sent to this recruiter from this user, user warmup cap hit (re-enqueued next day).

---

## 8. Scrapers — common interface

```go
type Scraper interface {
    Name() string
    Scrape(ctx context.Context, p ScrapeParams) ([]ScrapedJob, error)
}

type ScrapeParams struct {
    Titles    []string
    Locations []string
    Keywords  []string
    RemoteOnly bool
    Since     time.Time
}

type ScrapedJob struct {
    Source      string
    ExternalID  string
    CompanyName string
    CompanyDomain string
    Title       string
    Description string
    Location    string
    Remote      bool
    URL         string
    PostedAt    *time.Time
    Raw         json.RawMessage
}
```

### Source notes

- **Greenhouse:** `GET https://boards-api.greenhouse.io/v1/boards/{slug}/jobs?content=true`. Maintain a curated `companies_seed.csv` of company slugs. Add admin endpoint to extend list.
- **Lever:** `GET https://api.lever.co/v0/postings/{slug}?mode=json`. Same seed approach.
- **Ashby:** `POST https://api.ashbyhq.com/posting-api/job-board/{slug}` (no auth for public boards).
- **RemoteOK:** `GET https://remoteok.com/api` with `User-Agent`.
- **Wellfound:** has no stable public API. v1 = HTML scrape behind feature flag, mark fragile.
- **Indeed:** RSS at `/rss?q=...&l=...`. Description preview only, fetch detail page for full body. Backoff aggressively, they block.
- **Naukri:** HTML scrape. Mark fragile.
- **LinkedIn:** explicitly marked best-effort, will break. Use their public guest job search HTML. If anti-bot triggers, disable source for 24h and log. Do not engineer around captchas.

### Rate limits per source

Document in code as constants. Defaults: 1 req / 2s per source, 1 req / 5s for LinkedIn / Indeed / Naukri. Use `golang.org/x/time/rate` per-source limiter.

### Matching scraped jobs to preferences

In `worker-jobscraper` after insert, run match query against active campaigns:

```sql
-- pseudocode
match_score = 0
+ 0.4 if any pref.job_titles ILIKE %title%
+ 0.2 if any pref.job_functions match
+ 0.2 if location matches or remote
+ 0.1 for each pref.keywords_include present in description (cap 0.2)
- 0.5 if any pref.keywords_exclude present
```

Insert into `campaign_matches` if `score >= 0.4`. Tunable constant.

---

## 9. Email finder

### Strategy chain per `(job, company)` — first success wins, but record all

1. Check `recruiters` for `company_id` with `confidence != low`. If found, pick best.
2. If `company.careers_url` set, fetch it and `/about`, `/team`, `/contact`. Extract `mailto:` links matching `(jobs|careers|talent|recruit|hiring|hr)@<company-domain>` patterns. Confidence: high if role-prefixed, medium otherwise.
3. GitHub commit mining:
   - Resolve `company.github_org` from a curated map or by searching `https://api.github.com/search/users?q={company}+type:org`.
   - List repos, sort by `updated_at`, take top 10.
   - For each repo, `GET /repos/{org}/{repo}/commits?per_page=100&page=N` up to 3 pages.
   - Collect commit author emails, filter out `*@users.noreply.github.com`, `noreply@*`, `*@github.com`.
   - Confidence: low (we don't know if they're recruiters; document this clearly).
4. If everything fails, set `state=email_missing`. Admin can paste later via admin endpoint.

### Ethics + safety

- Hard cap: do not store more than 5 emails per company from GitHub mining.
- Honor robots.txt for careers page fetch (use a robots.txt cache).
- Skip emails matching `email_denylist`.
- Skip role aliases like `security@`, `abuse@`, `legal@`, `dmca@`, `privacy@`.

---

## 10. Content generation (Codex CLI) + anti-spam rules

### Invocation

`worker-contentgen` consumes `content:generate` jobs. For each, it:

1. Loads resume parsed text, job description, recruiter (name if any, company name), user preferences (tone preset + addendum).
2. Builds the prompt (below).
3. Invokes `codex exec` as subprocess with a strict timeout (`CODEX_TIMEOUT=60s`).
4. Parses the response (Codex returns JSON when prompted).
5. Inserts 3 variants into `email_drafts`, runs spam self-check, stores `spam_score`.
6. Picks variant with lowest `spam_score` (tie-break: shortest), marks `chosen=true`.
7. Transitions state to `generated`.

### Codex CLI invocation

```bash
codex exec --json --no-stream <<'PROMPT'
{... see below ...}
PROMPT
```

We invoke as the `kleos` system user. `CODEX_HOME` must be set so subscription auth is found. First-time setup documented in §16.

### Prompt template (verbatim string in `internal/contentgen/prompt.go`)

Variables filled by Go: `{{.ToneInstruction}}`, `{{.UserAddendum}}`, `{{.ResumeText}}`, `{{.JobTitle}}`, `{{.CompanyName}}`, `{{.JobDescription}}`, `{{.RecruiterName}}`.

```
You write outreach emails from a job candidate to a hiring contact at a company.
Output STRICT JSON only — no prose before or after. Schema:
{ "variants": [ { "subject": str, "body": str }, { "subject": str, "body": str }, { "subject": str, "body": str } ] }

HARD RULES (every variant must follow all):
1. Length: subject 4–9 words. Body 80–140 words. No exceptions.
2. Plain text only. No markdown. No emojis. No images. No HTML tags. No bullet lists. No tables.
3. Exactly zero links. Exactly zero phone numbers. Exactly zero attachments referenced.
4. No ALL CAPS words longer than 3 characters. No exclamation marks anywhere.
5. No words from this list anywhere (case-insensitive): free, guarantee, urgent, act now, limited time, click here, winner, congratulations, risk-free, no obligation, cash, bonus, opportunity of a lifetime, once in a lifetime, dear friend, dear sir or madam, to whom it may concern, hi dear, amazing, incredible, unbeatable, exclusive deal, special promotion, 100%, $$$, !!!.
6. No tracking language ("did you open this", "as you can see I").
7. Salutation: if RecruiterName is provided and non-empty, "Hi <FirstName>,". Otherwise "Hi <CompanyName> team,". Never "Dear Sir/Madam".
8. First sentence must reference one specific detail from the job description (technology, product area, or stated team challenge). Not generic praise of the company.
9. Second to fourth sentence: 2–3 concrete facts from the candidate's resume that map to the job. Use specific numbers when present (years, scale, metrics).
10. Closing sentence: a single specific ask — a short call or a reply at their convenience. Never two asks.
11. Sign-off: "Best, <CandidateFirstName>" on its own line. The candidate's first name is the first whitespace-separated token of the first non-empty line of the resume that looks like a name (capitalized words, no special characters).
12. Do not invent employment, degrees, or numbers not present in the resume.
13. Do not mention salary, visa, sponsorship, or relocation unless those words appear in the job description.
14. Each of the 3 variants must differ in: (a) the resume detail led with, (b) the closing ask phrasing, (c) the subject line. Avoid near-duplicates.

TONE: {{.ToneInstruction}}
{{ if .UserAddendum }}USER NOTE (apply only if it does not violate HARD RULES): {{.UserAddendum}}{{ end }}

CONTEXT:
RECRUITER_NAME: {{.RecruiterName}}
COMPANY_NAME: {{.CompanyName}}
JOB_TITLE: {{.JobTitle}}
JOB_DESCRIPTION:
"""
{{.JobDescription}}
"""

RESUME (plain text extracted from PDF):
"""
{{.ResumeText}}
"""

Return the JSON now and nothing else.
```

Tone presets map to `ToneInstruction`:
- `formal`: "Professional, courteous, no contractions. Sound like a written letter."
- `casual`: "Warm and natural. Contractions allowed. Sound like a thoughtful peer reaching out."
- `technical`: "Direct and specific. Lead with technical substance. Minimal pleasantries."
- `warm`: "Friendly but professional. Brief warmth, then substance. Default."

### Spam self-check (`spam_score`, 0.0–1.0, lower is better)

Implemented in Go after generation, **before** sending. Penalties added:

- +0.15 if any banned word slipped through (regex check, case-insensitive)
- +0.10 if subject > 9 words or < 4
- +0.10 if body > 160 or < 70 words
- +0.10 per `!`, per all-caps word (len > 3)
- +0.20 if any link present (regex `https?://`)
- +0.10 if no recruiter or company name present
- +0.05 per repeated 4-gram across variants (penalize sameness)

If `min(spam_score) > 0.30`, do **not** send. Set state to `failed` with reason `content_quality`, log, surface in UI. Re-generation can be requested manually via admin endpoint (post-v1).

---

## 11. Sender + warm-up

### Per-user state machine for warm-up

On first verified SMTP credential, create `warmup_state` row:
- `start_date = today`
- `current_day = 1`
- `todays_limit = WARMUP_DAY1_LIMIT` (5)
- `todays_sent = 0`

Daily rollover cron `warmup:rollover` at 00:05 UTC:
```
for each warmup_state where paused = false:
  if current_day < WARMUP_DAYS:
    current_day += 1
    todays_limit = min(WARMUP_CAP, ceil(WARMUP_DAY1_LIMIT * WARMUP_GROWTH^(current_day-1)))
  else:
    todays_limit = user.daily_send_cap  -- graduate
  todays_sent = 0
  last_rollover = today
```

With defaults: day 1=5, 2=7, 3=10, 4=14, 5=20, 6=27, 7=38, 8=40 (capped), 9..21=40. Graduates to user's `daily_send_cap` after day 21.

### Send loop (`worker-sender`)

For each `email:send` job:
1. Acquire per-user mutex (Redis SETNX with 5s TTL) to serialize sends per user.
2. Check `warmup_state.todays_sent < todays_limit`. If exceeded, reschedule task for next UTC day boundary. State remains `queued`.
3. Check per-user rate limits (hourly + daily) via sliding window in Redis sorted set keyed `ratelimit:send:{user_id}`.
4. Check `email_denylist` for recruiter email. If present, set state `skipped`, exit.
5. Check `sent_emails` uniqueness — should be impossible due to unique index, but guard.
6. Decrypt SMTP creds.
7. Build RFC 5322 message:
   - `From: "<from_name>" <from_email>`
   - `To: <recruiter_email>`
   - `Subject:` from draft
   - `Message-ID: <uuid@from_domain>`
   - `Date:` now
   - `MIME-Version: 1.0`
   - `Content-Type: text/plain; charset=UTF-8`
   - `Content-Transfer-Encoding: 8bit`
   - Plain text body only (HTML omitted in v1 — privacy + deliverability)
8. Open SMTP connection with STARTTLS (or implicit TLS on 465), AUTH PLAIN/LOGIN, send.
9. Sleep `rand(SEND_JITTER_MIN_SECONDS, SEND_JITTER_MAX_SECONDS)` **before** the send call to randomize timing. (Apply this delay inside the worker by scheduling the task with `ProcessIn`.)
10. On success: insert `sent_emails`, increment `warmup_state.todays_sent`, transition match to `sent`, audit-log.
11. On error: classify
    - 4xx → retry with backoff (asynq retries)
    - 5xx auth/sender → permanent fail, transition state to `failed`, set `smtp_credentials.last_error`, surface to user
    - 5xx recipient → permanent fail, add recipient to `email_denylist` with `reason='hard_bounce'`

### SMTP verification on add

`POST /api/smtp/{id}/verify` performs: connect → STARTTLS → AUTH → NOOP → QUIT. No actual send. Updates `verified_at`. Returns helpful error strings (DNS, auth, TLS).

---

## 12. Frontend (Next.js)

### Configuration
`next.config.js`:
```js
module.exports = {
  output: 'export',
  basePath: '/kleos',
  trailingSlash: true,
  images: { unoptimized: true },
}
```

### Pages and behavior

- `/` — marketing landing + login/signup tabs
- `/dashboard` — overview: counts (active campaigns, jobs scraped today, emails sent today, warmup status)
- `/onboarding/smtp` — add + verify SMTP, must complete to proceed
- `/onboarding/resume` — upload PDF, see parsed text preview, confirm
- `/onboarding/preferences` — titles, functions, level, locations, keywords, tone preset, custom addendum (textarea, max 500 chars)
- `/campaigns` — list with create button
- `/campaigns/[id]` — tabs: Matches | Drafts | Sent | Settings
  - Matches: paginated table grouped by state, filter chips
  - Drafts: read-only preview of chosen variant + 2 alternates
  - Sent: log with timestamps and SMTP response
  - Settings: rename, pause/resume, archive, change resume/SMTP
- `/settings` — change password, manage SMTP list, manage resumes, view warmup state

### API client
- `web/lib/api.ts` — fetch wrapper with auto-refresh on 401, typed via generated `web/lib/api.types.ts` (from Go OpenAPI spec — generate post-v1; for v1 hand-write types).
- Auth tokens in memory + refresh token in httpOnly cookie. Access token returned in body, kept in JS memory + localStorage.

### shadcn/ui components used
- `button`, `input`, `textarea`, `label`, `card`, `table`, `tabs`, `dialog`, `drawer`, `toast`, `badge`, `dropdown-menu`, `form` (react-hook-form), `select`, `switch`, `skeleton`

### State management
TanStack Query for server state. No global client state library — useState/useReducer per page.

### Build output
`npm run build` produces `web/out/`. nginx serves this directly (see §14).

---

## 13. Docker setup

### `deploy/Dockerfile.api` (multi-stage)

```dockerfile
FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
RUN apk add --no-cache ca-certificates poppler-utils tzdata && \
    adduser -D -u 10001 kleos
USER kleos
WORKDIR /app
COPY --from=build /out/api /app/api
EXPOSE 8080
ENTRYPOINT ["/app/api"]
```

### `deploy/Dockerfile.worker` (one image, build arg picks binary)

```dockerfile
FROM golang:1.23-alpine AS build
ARG WORKER
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/worker ./cmd/${WORKER}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates poppler-utils tzdata nodejs npm && \
    npm install -g @openai/codex-cli || true && \
    adduser -D -u 10001 kleos
# CODEX_HOME mounted as volume; subscription auth files placed there during VPS setup
USER kleos
WORKDIR /app
COPY --from=build /out/worker /app/worker
ENTRYPOINT ["/app/worker"]
```

Note on Codex CLI: only `worker-contentgen` actually needs it. Other workers can use a slimmer image. v1 trade-off: one image to simplify. Optimize later.

### `deploy/docker-compose.yml`

```yaml
name: kleos

services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: kleos
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: kleos
    volumes:
      - pgdata:/var/lib/postgresql/data
    # Do not publish to the host. The VPS already has a host Postgres on 127.0.0.1:5432.
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U kleos"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    restart: unless-stopped
    command: ["redis-server", "--save", "60", "1", "--appendonly", "yes"]
    volumes:
      - redisdata:/data
    # Do not publish to the host. The VPS already has a host Redis on 127.0.0.1:6379.
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  api:
    image: kleos/api:${TAG:-latest}
    restart: unless-stopped
    env_file: ../.env
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    volumes:
      - ../data/resumes:/data/resumes
      - ../logs:/var/log/kleos
    ports:
      - "127.0.0.1:8080:8080"

  worker-jobscraper:
    image: kleos/worker-jobscraper:${TAG:-latest}
    restart: unless-stopped
    env_file: ../.env
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    volumes:
      - ../logs:/var/log/kleos

  worker-emailfinder:
    image: kleos/worker-emailfinder:${TAG:-latest}
    restart: unless-stopped
    env_file: ../.env
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    volumes:
      - ../logs:/var/log/kleos

  worker-contentgen:
    image: kleos/worker-contentgen:${TAG:-latest}
    restart: unless-stopped
    env_file: ../.env
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    volumes:
      - ../data/resumes:/data/resumes:ro
      - ../codex-home:/home/kleos/.codex
      - ../logs:/var/log/kleos

  worker-sender:
    image: kleos/worker-sender:${TAG:-latest}
    restart: unless-stopped
    env_file: ../.env
    depends_on:
      postgres: { condition: service_healthy }
      redis:    { condition: service_healthy }
    volumes:
      - ../logs:/var/log/kleos

volumes:
  pgdata:
  redisdata:
```

Resource hints for 4GB / 2 vCPU VPS (add `deploy.resources` if needed):
- postgres ~512MB
- redis ~128MB
- api ~128MB
- each worker ~128MB (4 workers ~512MB)
- contentgen worker invokes Codex CLI as subprocess: budget extra ~512MB during invocations
- leave 1GB for OS + nginx + headroom

---

## 14. nginx

### Snippet included by the existing `abhiyadav.in` server block — `deploy/nginx.location.conf`

```nginx
# Include from /etc/nginx/snippets/site-locations.conf, matching the existing VPS pattern.

location = /kleos {
    return 301 /kleos/;
}

# Kleos frontend (Next.js static export)
location ^~ /kleos/ {
    alias /opt/kleos/web/;
    try_files $uri $uri/ $uri.html /kleos/index.html;
    add_header Cache-Control "public, max-age=300";
}

# Kleos API
location ^~ /kleos/api/ {
    proxy_pass http://127.0.0.1:8080/;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 90s;
    client_max_body_size 12m;        # resume PDFs
}
```

Install once as `/etc/nginx/snippets/kleos.locations.conf`, add `include /etc/nginx/snippets/kleos.locations.conf;` to `/etc/nginx/snippets/site-locations.conf`, then validate with `nginx -t && systemctl reload nginx`.

---

## 15. CI/CD — GitHub Actions

GitHub repository name: `Kleos`.

Required GitHub Actions secrets:

| Secret | Value |
|---|---|
| `VPS_HOST` | `abhiyadav.in` |
| `VPS_PORT` | `2022` |
| `VPS_USER` | `deploy` |
| `VPS_SSH_KEY` | private key whose public key is installed in `/home/deploy/.ssh/authorized_keys` |

### `.github/workflows/ci.yml` (PRs)

```yaml
name: ci
on:
  pull_request:
    branches: [main]
jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }
      - run: go vet ./...
      - uses: golangci/golangci-lint-action@v6
        with: { version: v1.60 }
      - run: go test ./... -race -count=1
  web:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with: { node-version: '20', cache: 'npm', cache-dependency-path: 'web/package-lock.json' }
      - run: npm ci
        working-directory: web
      - run: npm run lint
        working-directory: web
      - run: npm run build
        working-directory: web
```

### `.github/workflows/deploy.yml` (main)

```yaml
name: deploy
on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    concurrency:
      group: deploy-prod
      cancel-in-progress: false
    steps:
      - uses: actions/checkout@v4

      - name: Set TAG
        run: echo "TAG=$(git rev-parse --short HEAD)" >> $GITHUB_ENV

      - uses: actions/setup-go@v5
        with: { go-version: '1.23', cache: true }

      - name: Build Go binaries
        run: |
          mkdir -p dist
          for c in api worker-jobscraper worker-emailfinder worker-contentgen worker-sender migrate; do
            CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
              go build -ldflags="-s -w -X main.version=${TAG}" -o dist/$c ./cmd/$c
          done

      - uses: actions/setup-node@v4
        with: { node-version: '20', cache: 'npm', cache-dependency-path: 'web/package-lock.json' }

      - name: Build frontend
        working-directory: web
        run: |
          npm ci
          npm run build

      - name: Stage artifacts
        run: |
          mkdir -p stage/{bin,web,migrations,deploy}
          cp dist/* stage/bin/
          cp -r web/out/* stage/web/
          cp -r migrations/* stage/migrations/
          cp -r deploy/* stage/deploy/

      - name: Setup SSH
        env:
          SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
          HOST: ${{ secrets.VPS_HOST }}
          PORT: ${{ secrets.VPS_PORT }}
        run: |
          mkdir -p ~/.ssh
          echo "$SSH_KEY" > ~/.ssh/id_ed25519
          chmod 600 ~/.ssh/id_ed25519
          ssh-keyscan -p "$PORT" -H "$HOST" >> ~/.ssh/known_hosts

      - name: Rsync to VPS
        env:
          HOST: ${{ secrets.VPS_HOST }}
          PORT: ${{ secrets.VPS_PORT }}
          USER: ${{ secrets.VPS_USER }}
        run: |
          rsync -az --delete -e "ssh -p $PORT" stage/bin/        $USER@$HOST:/opt/kleos/bin.new/
          rsync -az --delete -e "ssh -p $PORT" stage/web/        $USER@$HOST:/opt/kleos/web.new/
          rsync -az --delete -e "ssh -p $PORT" stage/migrations/ $USER@$HOST:/opt/kleos/migrations/
          rsync -az          -e "ssh -p $PORT" stage/deploy/     $USER@$HOST:/opt/kleos/deploy/

      - name: Run deploy.sh on VPS
        env:
          HOST: ${{ secrets.VPS_HOST }}
          PORT: ${{ secrets.VPS_PORT }}
          USER: ${{ secrets.VPS_USER }}
        run: |
          ssh -p "$PORT" $USER@$HOST "TAG=${TAG} bash /opt/kleos/deploy/deploy.sh"
```

### `deploy/deploy.sh` (runs on VPS, idempotent)

```bash
#!/usr/bin/env bash
set -euo pipefail

APP_DIR=/opt/kleos
cd "$APP_DIR"

# 1. Build local images from rsynced binaries
# Strategy: package each Go binary into a tiny image. We use Dockerfiles in deploy/.
# For speed we use a single "runtime" Dockerfile and pass BINARY as build arg.

cat > /tmp/Dockerfile.runtime <<'EOF'
FROM alpine:3.20
RUN apk add --no-cache ca-certificates poppler-utils tzdata
ARG BINARY
COPY ${BINARY} /app/binary
RUN adduser -D -u 10001 kleos && chown kleos /app/binary
USER kleos
WORKDIR /app
ENTRYPOINT ["/app/binary"]
EOF

# Special image for contentgen with Node + Codex CLI
cat > /tmp/Dockerfile.contentgen <<'EOF'
FROM alpine:3.20
RUN apk add --no-cache ca-certificates poppler-utils tzdata nodejs npm
RUN npm install -g @openai/codex
ARG BINARY
COPY ${BINARY} /app/binary
RUN adduser -D -u 10001 kleos && chown kleos /app/binary
USER kleos
WORKDIR /app
ENTRYPOINT ["/app/binary"]
EOF

cd "$APP_DIR/bin.new"

docker build -t kleos/api:${TAG}                -f /tmp/Dockerfile.runtime    --build-arg BINARY=api .
docker build -t kleos/worker-jobscraper:${TAG}  -f /tmp/Dockerfile.runtime    --build-arg BINARY=worker-jobscraper .
docker build -t kleos/worker-emailfinder:${TAG} -f /tmp/Dockerfile.runtime    --build-arg BINARY=worker-emailfinder .
docker build -t kleos/worker-sender:${TAG}      -f /tmp/Dockerfile.runtime    --build-arg BINARY=worker-sender .
docker build -t kleos/worker-contentgen:${TAG}  -f /tmp/Dockerfile.contentgen --build-arg BINARY=worker-contentgen .

cd "$APP_DIR"

# 2. Bring up DB/Redis first so the compose network exists, then run migrations.
docker compose -f "$APP_DIR/deploy/docker-compose.yml" --env-file "$APP_DIR/.env" up -d postgres redis

docker run --rm \
  --env-file "$APP_DIR/.env" \
  --network kleos_default \
  -v "$APP_DIR/migrations:/migrations:ro" \
  -v "$APP_DIR/bin.new:/bin:ro" \
  --entrypoint /bin/migrate \
  alpine:3.20

# 3. Swap web dir
mv "$APP_DIR/web" "$APP_DIR/web.old.$$" 2>/dev/null || true
mv "$APP_DIR/web.new" "$APP_DIR/web"
rm -rf "$APP_DIR/web.old.$$" 2>/dev/null || true

# 4. Re-up compose with new image tags
TAG=${TAG} docker compose -f "$APP_DIR/deploy/docker-compose.yml" --env-file "$APP_DIR/.env" up -d

# 5. Clean up bin.new (binaries baked into images already)
rm -rf "$APP_DIR/bin.new"

# 6. Prune old images (keep last 3)
docker image prune -f
echo "deploy ok: TAG=${TAG}"
```

The migration path is intentionally deterministic: start Postgres/Redis, wait for their compose healthchecks, then run the one-shot migration container on `kleos_default`.

---

## 16. VPS first-time setup runbook

Run once, manually from the local machine with `ssh vps`. That alias resolves to `abhinav@abhiyadav.in -p 2022`; the current `abhinav` user has passwordless sudo but is not in the Docker group.

```bash
# Packages/runtime
sudo apt update
sudo apt install -y docker.io docker-compose-v2 rsync git ca-certificates
sudo systemctl enable --now docker

# Create CI deploy user
getent passwd deploy >/dev/null || sudo useradd -m -s /bin/bash deploy
sudo usermod -aG docker deploy

# App dir
sudo install -d -o deploy -g deploy /opt/kleos
sudo -u deploy mkdir -p /opt/kleos/{data/resumes,logs,codex-home,migrations,deploy,web,bin.new,backups,scripts}

# SSH key for GitHub Actions
sudo -u deploy mkdir -p /home/deploy/.ssh
sudo -u deploy chmod 700 /home/deploy/.ssh
sudo -u deploy touch /home/deploy/.ssh/authorized_keys
sudo -u deploy chmod 600 /home/deploy/.ssh/authorized_keys
# append the CI public key that matches GitHub secret VPS_SSH_KEY

# .env: create with values from .env.example, storing real secrets only on VPS
sudo -u deploy install -m 600 /dev/null /opt/kleos/.env

# Logrotate
sudo tee /etc/logrotate.d/kleos >/dev/null <<'EOF'
/opt/kleos/logs/*.log {
  daily
  rotate 14
  compress
  missingok
  notifempty
  copytruncate
}
EOF

# Codex CLI subscription auth (interactive, once)
sudo chown -R 10001:10001 /opt/kleos/codex-home
sudo -u deploy bash -lc '
  docker run -it --rm \
    --user 10001:10001 \
    -v /opt/kleos/codex-home:/home/kleos/.codex \
    -e HOME=/home/kleos \
    -e NPM_CONFIG_PREFIX=/tmp/npm \
    -e PATH=/tmp/npm/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    node:20-alpine sh -c "npm i -g @openai/codex && codex login"
'

# nginx
sudo tee /etc/nginx/snippets/kleos.locations.conf >/dev/null <<'EOF'
location = /kleos {
    return 301 /kleos/;
}

location ^~ /kleos/ {
    alias /opt/kleos/web/;
    try_files $uri $uri/ $uri.html /kleos/index.html;
    add_header Cache-Control "public, max-age=300";
}

location ^~ /kleos/api/ {
    proxy_pass http://127.0.0.1:8080/;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_read_timeout 90s;
    client_max_body_size 12m;
}
EOF
sudo grep -q 'kleos.locations.conf' /etc/nginx/snippets/site-locations.conf \
  || echo 'include /etc/nginx/snippets/kleos.locations.conf;' | sudo tee -a /etc/nginx/snippets/site-locations.conf
sudo nginx -t
sudo systemctl reload nginx
```

Then push to `main` once → CI deploys everything else.

---

## 17. Logging

- All Go services use `log/slog` with JSON handler writing to `/var/log/kleos/<service>.log` (mounted volume).
- Each log line includes: `time, level, msg, service, request_id (if http), user_id (if known), task_id (if worker)`.
- HTTP middleware sets `request_id` (UUIDv7) and logs `start` and `end` with status + latency.
- Worker middleware logs `task_started` and `task_completed/failed` with duration.
- No PII in logs at info level. Email addresses logged at debug only.
- Logrotate handles rotation (configured in §16).

---

## 18. Security checklist (must be done in v1)

- [ ] All SMTP passwords AES-256-GCM encrypted at rest, key only in env
- [ ] Bcrypt cost 12 for user passwords
- [ ] JWT secret 64 bytes, rotated by deploying new value (logs out everyone, document)
- [ ] CSRF: APIs only accept `Bearer` tokens, no cookie-auth state-changing endpoints
- [ ] CORS: API allows only `https://abhiyadav.in`
- [ ] Rate limit middleware: 60 req/min/IP for unauth, 600 req/min/user for auth, 5/min for `/auth/login` and `/auth/signup`
- [ ] Resume upload: enforce `application/pdf`, max 8 MB, reject if pdftotext returns empty (likely scanned image)
- [ ] No path traversal in resume storage path (use UUIDs)
- [ ] Postgres + Redis are not published to host ports in prod; only API publishes `127.0.0.1:8080`
- [ ] Docker user is non-root in all images
- [ ] `email_denylist` honored everywhere a send is enqueued AND before send
- [ ] Hard unique constraint on `(user_id, recruiter_email)` in `sent_emails` prevents accidental dupes
- [ ] Admin endpoints require `is_admin=true`, gated by middleware

---

## 19. Acceptance criteria per phase

Agent must verify each box before moving to next phase.

### Phase 0 — Foundation
- [x] Repo initialized, `go.mod`, `golangci-lint` config, `Makefile` with `lint`, `test`, `build`, `up`, `down`
- [x] `docker-compose.dev.yml` brings up Postgres + Redis only, app runs on host
- [x] Migration 001 applies cleanly via `cmd/migrate`
- [x] `GET /api/healthz` returns 200
- [x] `GET /api/readyz` checks DB + Redis, returns 200/503
- [x] CI workflow runs vet + lint + test on PR
- [x] First successful deploy from `main` lands at `https://abhiyadav.in/kleos/api/healthz`

### Phase 1 — Auth + SMTP + Resume + Preferences
- [x] Signup, login, refresh, logout work end-to-end
- [ ] `POST /api/smtp` stores AES-GCM ciphertext; verify endpoint succeeds against Gmail app password and a custom server
- [ ] Resume upload validates PDF, runs pdftotext, stores file + parsed text
- [ ] Preferences CRUD round-trips arrays and tone preset

### Phase 2 — Job scraping
- [ ] Migrations 002 applied
- [ ] Greenhouse, Lever, Ashby, RemoteOK scrapers return ≥10 jobs each against seed companies
- [ ] Indeed, Naukri, Wellfound, LinkedIn implementations exist, marked fragile, gracefully skip on failure
- [ ] Dedup by `(source, external_id)` works
- [ ] `campaign:tick` creates `campaign_matches` rows; match score formula implemented
- [ ] Per-source rate limiters enforce documented limits

### Phase 3 — Email finder
- [ ] `recruiters` populated from careers page mailto for ≥5 companies in test fixture
- [ ] GitHub commit mining populates with `confidence=low`
- [ ] Admin paste endpoint inserts manual recruiters
- [ ] State transitions `new → finding_email → email_found|email_missing` observed in DB
- [ ] Role aliases (`security@`, etc.) filtered
- [ ] Denylist honored

### Phase 4 — Content generation
- [ ] Codex CLI invoked from worker, returns valid JSON for ≥10 test (resume, JD) pairs
- [ ] Prompt enforces all 14 hard rules — spot-check 5 outputs manually
- [ ] Spam self-check computes scores; bad outputs get `state=failed` with `reason=content_quality`
- [ ] 3 variants stored per match, one marked `chosen`

### Phase 5 — Sender + warm-up
- [ ] `warmup_state` row auto-created on first verified SMTP
- [ ] Day-N limit formula matches §11 table
- [ ] Sends happen with random jitter between 30–180s
- [ ] Hard bounces add recipient to `email_denylist`
- [ ] Unique constraint prevents dupes
- [ ] `sent_emails.message_id` recorded; `smtp_response` captured
- [ ] Permanent SMTP auth failure pauses the credential, surfaces in UI

### Phase 6 — Frontend
- [ ] Static export builds, served at `https://abhiyadav.in/kleos/`
- [ ] All pages listed in §12 implemented
- [ ] Token refresh works on 401
- [ ] Resume PDF upload UI works against API
- [ ] Campaign create → matches visible within 15 minutes of `campaign:tick`

### Phase 7 — Hardening
- [ ] Rate limit middleware verified with a load test
- [ ] Audit log entries written for: signup, login, smtp add/delete, send success/fail, admin actions
- [ ] Logrotate config present and rotates a sample log
- [ ] Backup script `pg_dump | gzip > /opt/kleos/backups/$(date).sql.gz` on daily cron, retain 14 days
- [ ] Disaster recovery: documented restore procedure tested against a fresh container

---

## 20. Out of scope for v1 (do not build)

- Follow-up sequences (multi-touch outreach). Schema accommodates them via future `follow_up_of` column on `sent_emails`; defer.
- Open/click/reply tracking
- Shared SMTP pool / managed sending domain
- Paid email finder APIs (hunter.io, Apollo)
- Reply inbox / unified IMAP
- Team accounts / multi-user campaigns
- A/B testing of variants beyond the choose-lowest-spam-score logic
- Mobile app

---

## 21. Open questions to revisit before launch

1. Legal review of GitHub commit email use for outreach — confirm with a lawyer for India + US recipients. Add a Terms of Service that the user attests they have a lawful basis to contact each recruiter.
2. Privacy policy + data deletion endpoint (GDPR-style export/delete) — required before public signup.
3. CAN-SPAM / DPDP compliance: even without tracking, the email body must include user's real identity. The prompt enforces a signed sign-off; verify final outputs comply.
4. Codex CLI usage limits under subscription — if 1 generation per match × many matches × many users hits the cap, fall back to caching prompts per (resume_hash, job_id) pair.
5. LinkedIn scraping legal exposure — keep it behind a feature flag default-off in v1.

---

## 22. Daily ops cheat-sheet (for me, post-launch)

```bash
# tail logs
docker compose -f /opt/kleos/deploy/docker-compose.yml logs -f api worker-sender

# pause a user (admin)
psql ... -c "UPDATE users SET is_active=false WHERE email='x@y.z';"

# pause warmup for a user
psql ... -c "UPDATE warmup_state SET paused=true WHERE user_id='...';"

# add to denylist
curl -X POST .../api/admin/denylist -d '{"email":"...","reason":"complaint"}'

# rotate JWT secret
# - update .env on VPS
# - docker compose restart api
# - all users get logged out (expected)

# backup
/opt/kleos/scripts/backup.sh

# restore
gunzip -c /opt/kleos/backups/YYYY-MM-DD.sql.gz | docker exec -i $(docker compose -f /opt/kleos/deploy/docker-compose.yml ps -q postgres) psql -U kleos
```

---

End of plan.md.

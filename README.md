# Kleos

**Kleos helps job seekers run structured, privacy-first outreach from their own email account.**

Kleos is a public-signup SaaS for job outreach. Users create an account, connect their own SMTP sender, upload a resume PDF, set search preferences, and use the system to organize job discovery and recruiter outreach from a single dashboard.

Live app: [https://abhiyadav.in/kleos/](https://abhiyadav.in/kleos/)

API base: [https://abhiyadav.in/kleos/api/](https://abhiyadav.in/kleos/api/)

## Current Status

The foundation and onboarding APIs are live:

- Public API health checks are deployed.
- User signup, login, refresh, logout, and current-user APIs are live.
- SMTP credential storage uses AES-GCM encryption at rest.
- SMTP verification works with a custom SMTP server.
- Resume PDF upload, text extraction, activation, listing, and deletion are live.
- Preferences GET and full-replace PUT are live.
- A read-only Progress + Health dashboard is served at `/kleos/`.

Remaining setup item: Gmail SMTP app-password verification.

## User Flow

1. Open [https://abhiyadav.in/kleos/](https://abhiyadav.in/kleos/).
2. Create an account when signup UI is enabled.
3. Add an SMTP sender:
   - host
   - port
   - username
   - app password or SMTP password
   - from email
   - from name
   - TLS setting
4. Upload a PDF resume.
5. Set role, location, keyword, remote, seniority, and tone preferences.
6. Create campaigns when the job-scraping phase is enabled.

## Features

- Public signup account model.
- JWT access tokens with refresh-token rotation.
- User-owned SMTP sending.
- Encrypted SMTP passwords at rest.
- SMTP verification endpoint.
- Primary SMTP sender selection.
- Resume PDF parsing with `pdftotext`.
- Preferences API for titles, functions, seniority, locations, keywords, remote-only filtering, tone preset, and custom addendum.
- Static dashboard for project progress and API health.
- Single-VPS deployment with Docker Compose and nginx.

## Technical Overview

- Backend: Go, `chi`, PostgreSQL, Redis.
- Auth: bcrypt password hashing, HS256 JWTs, refresh-token rotation.
- Database migrations: `goose`.
- SMTP storage: AES-256-GCM with key from environment.
- Resume parsing: `pdftotext` from `poppler-utils`.
- Runtime: Docker Compose on a VPS.
- Reverse proxy: nginx at `/kleos/` and `/kleos/api/`.
- CI/CD: GitHub Actions builds binaries, rsyncs artifacts to the VPS, and runs `deploy/deploy.sh`.
- Frontend: static HTML/CSS/JavaScript served by nginx.

## Repository Layout

```text
cmd/
  api/                 HTTP API server
  migrate/             Database migration runner
deploy/
  docker-compose.yml   Production services
  deploy.sh            VPS deployment script
  nginx.location.conf  nginx locations for /kleos/
internal/
  auth/                Authentication and token services
  config/              Environment configuration
  crypto/              AES-GCM helpers
  db/                  PostgreSQL and Redis connections
  http/                HTTP routes
  preferences/         User preferences service
  resume/              Resume storage and parsing service
  smtpcred/            SMTP credential service
migrations/            SQL migrations
web/                   Static Progress + Health dashboard
```

## Local Setup

Requirements:

- Go 1.23 or newer
- Docker and Docker Compose
- PostgreSQL and Redis through `deploy/docker-compose.dev.yml`

Clone and configure:

```bash
git clone git@github.com:abhinav-yadav-official/Kleos.git
cd Kleos
cp .env.example .env
```

Start local dependencies:

```bash
make up
```

Run migrations:

```bash
make migrate
```

Run the API:

```bash
make run-api
```

Open local API checks:

```bash
curl http://127.0.0.1:8080/api/healthz
curl http://127.0.0.1:8080/api/readyz
```

Serve the static dashboard locally:

```bash
python3 -m http.server 8765 --directory web
```

Then open [http://127.0.0.1:8765/](http://127.0.0.1:8765/).

## Development Commands

```bash
make test    # go test ./... -count=1
make build   # build api and migrate binaries into bin/
make lint    # go vet ./...
make up      # start local Postgres and Redis
make down    # stop local Postgres and Redis
```

## Environment

Key environment variables:

```env
APP_ENV=development
APP_PORT=8080
APP_BASE_URL=http://localhost:8080
DB_DSN=postgres://kleos:kleos@localhost:5433/kleos?sslmode=disable
REDIS_ADDR=localhost:6380
JWT_SECRET=CHANGE_ME_64_BYTES_HEX
SMTP_CRED_ENCRYPTION_KEY=CHANGE_ME_32_BYTES_HEX
RESUME_STORAGE_DIR=./data/resumes
```

Generate production secrets with:

```bash
openssl rand -hex 32
```

## Deployment

Production deployment targets the existing VPS behind `abhiyadav.in`.

Public routes:

- `https://abhiyadav.in/kleos/` serves static dashboard files from `/opt/kleos/web/`.
- `https://abhiyadav.in/kleos/api/` proxies to the Go API on `127.0.0.1:8080`.

The GitHub Actions deploy workflow:

1. Builds Linux Go binaries for `api` and `migrate`.
2. Copies `web/`, `migrations/`, `deploy/`, and binaries to `/opt/kleos`.
3. Runs `deploy/deploy.sh` over SSH.
4. Starts or updates the API container with Docker Compose.

Manual deploy from the VPS:

```bash
cd /opt/kleos
TAG=manual bash deploy/deploy.sh
```

Required GitHub Actions secrets:

- `VPS_HOST`
- `VPS_PORT`
- `VPS_USER`
- `VPS_SSH_KEY`

## API Smoke Checks

```bash
curl -fsS https://abhiyadav.in/kleos/api/healthz
curl -fsS https://abhiyadav.in/kleos/api/readyz
curl -fsS https://abhiyadav.in/kleos/
```

## License

MIT License. Copyright (c) 2026 Abhinav Yadav.

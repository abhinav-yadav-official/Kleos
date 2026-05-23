-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE companies (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name          TEXT NOT NULL,
  slug          TEXT UNIQUE NOT NULL,
  domain        TEXT,
  careers_url   TEXT,
  github_org    TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_companies_domain ON companies(domain);

CREATE TABLE jobs (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source         TEXT NOT NULL,
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

CREATE TABLE recruiters (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  company_id    UUID REFERENCES companies(id),
  email         CITEXT NOT NULL,
  name          TEXT,
  title         TEXT,
  source        TEXT NOT NULL,
  confidence    TEXT NOT NULL,
  evidence_url  TEXT,
  is_blocked    BOOLEAN NOT NULL DEFAULT false,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (email, company_id)
);
CREATE INDEX idx_recruiters_company ON recruiters(company_id);

CREATE TABLE email_denylist (
  email      CITEXT PRIMARY KEY,
  reason     TEXT,
  added_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS email_denylist;
DROP INDEX IF EXISTS idx_recruiters_company;
DROP TABLE IF EXISTS recruiters;
DROP INDEX IF EXISTS idx_jobs_title_trgm;
DROP INDEX IF EXISTS idx_jobs_scraped_at;
DROP INDEX IF EXISTS idx_jobs_company;
DROP TABLE IF EXISTS jobs;
DROP INDEX IF EXISTS idx_companies_domain;
DROP TABLE IF EXISTS companies;
DROP EXTENSION IF EXISTS pg_trgm;

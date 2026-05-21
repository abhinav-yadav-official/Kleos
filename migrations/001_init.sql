-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE users (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email          CITEXT UNIQUE NOT NULL,
  password_hash  TEXT NOT NULL,
  name           TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  is_active      BOOLEAN NOT NULL DEFAULT true,
  is_admin       BOOLEAN NOT NULL DEFAULT false,
  daily_send_cap INT NOT NULL DEFAULT 100,
  hourly_send_cap INT NOT NULL DEFAULT 20
);

CREATE TABLE refresh_tokens (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,
  expires_at  TIMESTAMPTZ NOT NULL,
  revoked_at  TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_refresh_tokens_user ON refresh_tokens(user_id);

CREATE TABLE smtp_credentials (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label           TEXT NOT NULL,
  host            TEXT NOT NULL,
  port            INT NOT NULL,
  username        TEXT NOT NULL,
  password_cipher BYTEA NOT NULL,
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
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  filename     TEXT NOT NULL,
  storage_path TEXT NOT NULL,
  parsed_text  TEXT NOT NULL,
  is_active    BOOLEAN NOT NULL DEFAULT true,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_resumes_user_active ON resumes(user_id) WHERE is_active;

CREATE TABLE preferences (
  user_id          UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  job_titles       TEXT[] NOT NULL DEFAULT '{}',
  job_functions    TEXT[] NOT NULL DEFAULT '{}',
  experience_level TEXT NOT NULL DEFAULT 'mid',
  locations        TEXT[] NOT NULL DEFAULT '{}',
  keywords_include TEXT[] NOT NULL DEFAULT '{}',
  keywords_exclude TEXT[] NOT NULL DEFAULT '{}',
  remote_only      BOOLEAN NOT NULL DEFAULT false,
  tone_preset      TEXT NOT NULL DEFAULT 'warm',
  tone_addendum    TEXT NOT NULL DEFAULT '',
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS preferences;
DROP TABLE IF EXISTS resumes;
DROP INDEX IF EXISTS idx_smtp_one_primary_per_user;
DROP TABLE IF EXISTS smtp_credentials;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
DROP EXTENSION IF EXISTS citext;
DROP EXTENSION IF EXISTS pgcrypto;

-- +goose Up
CREATE TABLE campaigns (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL,
  status       TEXT NOT NULL DEFAULT 'active',
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
  UNIQUE (campaign_id, job_id)
);
CREATE INDEX idx_matches_state ON campaign_matches(state);
CREATE INDEX idx_matches_campaign ON campaign_matches(campaign_id);

CREATE TABLE email_drafts (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  match_id        UUID NOT NULL REFERENCES campaign_matches(id) ON DELETE CASCADE,
  recruiter_id    UUID NOT NULL REFERENCES recruiters(id),
  variant         INT NOT NULL,
  subject         TEXT NOT NULL,
  body_text       TEXT NOT NULL,
  body_html       TEXT,
  chosen          BOOLEAN NOT NULL DEFAULT false,
  spam_score      REAL,
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
  message_id      TEXT NOT NULL,
  status          TEXT NOT NULL,
  smtp_response   TEXT,
  sent_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sent_user_time ON sent_emails(user_id, sent_at DESC);
CREATE INDEX idx_sent_recruiter ON sent_emails(recruiter_email);
CREATE UNIQUE INDEX uniq_sent_per_user_per_recruiter
  ON sent_emails(user_id, recruiter_email);

-- +goose Down
DROP INDEX IF EXISTS uniq_sent_per_user_per_recruiter;
DROP INDEX IF EXISTS idx_sent_recruiter;
DROP INDEX IF EXISTS idx_sent_user_time;
DROP TABLE IF EXISTS sent_emails;
DROP INDEX IF EXISTS idx_drafts_match;
DROP TABLE IF EXISTS email_drafts;
DROP INDEX IF EXISTS idx_matches_campaign;
DROP INDEX IF EXISTS idx_matches_state;
DROP TABLE IF EXISTS campaign_matches;
DROP INDEX IF EXISTS idx_campaigns_user;
DROP TABLE IF EXISTS campaigns;

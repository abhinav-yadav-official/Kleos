-- +goose Up
ALTER TABLE email_drafts ADD COLUMN resume_hash TEXT;
CREATE INDEX idx_email_drafts_cache ON email_drafts(resume_hash, recruiter_id) WHERE resume_hash IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_email_drafts_cache;
ALTER TABLE email_drafts DROP COLUMN resume_hash;

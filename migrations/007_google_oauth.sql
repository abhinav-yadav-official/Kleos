-- +goose Up
ALTER TABLE users ADD COLUMN google_sub TEXT UNIQUE;
ALTER TABLE users ADD COLUMN email_verified_at TIMESTAMPTZ;
-- Password hash is now optional so Google-only accounts can sign up without
-- ever setting a password.
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;

-- +goose Down
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;
ALTER TABLE users DROP COLUMN IF EXISTS google_sub;

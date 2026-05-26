-- +goose Up
ALTER TABLE users ADD COLUMN tos_accepted_at TIMESTAMPTZ;

ALTER TABLE audit_log DROP CONSTRAINT audit_log_user_id_fkey;
ALTER TABLE audit_log
  ADD CONSTRAINT audit_log_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE sent_emails DROP CONSTRAINT sent_emails_user_id_fkey;
ALTER TABLE sent_emails
  ADD CONSTRAINT sent_emails_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
ALTER TABLE sent_emails DROP CONSTRAINT sent_emails_user_id_fkey;
ALTER TABLE sent_emails
  ADD CONSTRAINT sent_emails_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE audit_log DROP CONSTRAINT audit_log_user_id_fkey;
ALTER TABLE audit_log
  ADD CONSTRAINT audit_log_user_id_fkey
  FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE users DROP COLUMN tos_accepted_at;

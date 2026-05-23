-- +goose Up
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
  actor      TEXT NOT NULL,
  action     TEXT NOT NULL,
  target     TEXT,
  meta       JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_user_time ON audit_log(user_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_audit_user_time;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS warmup_state;

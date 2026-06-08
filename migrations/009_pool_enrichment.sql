-- +goose Up
ALTER TABLE recruiters ADD COLUMN IF NOT EXISTS phone TEXT;
ALTER TABLE recruiters ADD COLUMN IF NOT EXISTS mobile TEXT;

ALTER TABLE companies ADD COLUMN IF NOT EXISTS company_type TEXT;
ALTER TABLE companies ADD COLUMN IF NOT EXISTS company_size TEXT;
ALTER TABLE companies ADD COLUMN IF NOT EXISTS company_location TEXT;

-- +goose Down
ALTER TABLE recruiters DROP COLUMN IF EXISTS phone;
ALTER TABLE recruiters DROP COLUMN IF EXISTS mobile;

ALTER TABLE companies DROP COLUMN IF EXISTS company_type;
ALTER TABLE companies DROP COLUMN IF EXISTS company_size;
ALTER TABLE companies DROP COLUMN IF EXISTS company_location;

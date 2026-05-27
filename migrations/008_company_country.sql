-- +goose Up
ALTER TABLE companies ADD COLUMN country TEXT;
CREATE INDEX idx_companies_country ON companies(country) WHERE country IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_companies_country;
ALTER TABLE companies DROP COLUMN IF EXISTS country;

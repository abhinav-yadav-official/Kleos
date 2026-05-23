package scraper

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

var slugCleanRE = regexp.MustCompile(`[^a-z0-9]+`)

func NormalizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugCleanRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// UpsertCompany returns the company id for the given slug, inserting a row if
// none exists. Name is used only on insert; existing rows are left untouched.
func (s *Store) UpsertCompany(ctx context.Context, name, slug, domain string) (string, error) {
	slug = NormalizeSlug(slug)
	if slug == "" {
		return "", errors.New("empty slug")
	}
	if name == "" {
		name = slug
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO companies (name, slug, domain)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (slug) DO UPDATE SET slug = EXCLUDED.slug
		RETURNING id::text
	`, name, slug, domain).Scan(&id)
	return id, err
}

// UpsertJob inserts a scraped job (or updates fields if (source, external_id)
// already exists). Returns the job id and whether the row was newly inserted.
func (s *Store) UpsertJob(ctx context.Context, companyID string, j ScrapedJob) (id string, inserted bool, err error) {
	raw := j.Raw
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	row := s.pool.QueryRow(ctx, `
		INSERT INTO jobs (source, external_id, company_id, title, description,
			location, remote, url, posted_at, raw)
		VALUES ($1, $2, $3::uuid, $4, $5, $6, $7, $8, $9, $10::jsonb)
		ON CONFLICT (source, external_id) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			location = EXCLUDED.location,
			remote = EXCLUDED.remote,
			url = EXCLUDED.url,
			posted_at = EXCLUDED.posted_at,
			raw = EXCLUDED.raw,
			scraped_at = now()
		RETURNING id::text, (xmax = 0) AS inserted
	`,
		j.Source, j.ExternalID, companyID, j.Title, j.Description,
		nullIfEmpty(j.Location), j.Remote, j.URL, j.PostedAt, raw,
	)
	err = row.Scan(&id, &inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, err
	}
	return id, inserted, err
}

// PersistAll persists a batch of scraped jobs, returning counts of inserted vs
// updated rows. Failures on individual jobs are returned via the first error.
func (s *Store) PersistAll(ctx context.Context, jobs []ScrapedJob) (insertedCount, updatedCount int, err error) {
	for _, j := range jobs {
		companyID, e := s.UpsertCompany(ctx, j.CompanyName, j.CompanySlug, j.CompanyDomain)
		if e != nil {
			return insertedCount, updatedCount, e
		}
		_, inserted, e := s.UpsertJob(ctx, companyID, j)
		if e != nil {
			return insertedCount, updatedCount, e
		}
		if inserted {
			insertedCount++
		} else {
			updatedCount++
		}
	}
	return insertedCount, updatedCount, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

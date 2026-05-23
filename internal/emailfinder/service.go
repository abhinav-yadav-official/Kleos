package emailfinder

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool     *pgxpool.Pool
	mailto   *MailtoExtractor
	github   *GitHubMiner
}

func NewService(pool *pgxpool.Pool, mailto *MailtoExtractor, github *GitHubMiner) *Service {
	return &Service{pool: pool, mailto: mailto, github: github}
}

// Company is the minimum company shape the finder needs.
type Company struct {
	ID         string
	Name       string
	Slug       string
	Domain     string
	CareersURL string
	GitHubOrg  string
}

var ErrCompanyNotFound = errors.New("company not found")

func (s *Service) GetCompany(ctx context.Context, id string) (Company, error) {
	var c Company
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, name, slug, COALESCE(domain,''), COALESCE(careers_url,''), COALESCE(github_org,'')
		FROM companies WHERE id = $1::uuid
	`, id).Scan(&c.ID, &c.Name, &c.Slug, &c.Domain, &c.CareersURL, &c.GitHubOrg)
	if errors.Is(err, pgx.ErrNoRows) {
		return Company{}, ErrCompanyNotFound
	}
	return c, err
}

// FindForCompany runs the strategy chain (existing recruiters → mailto → github)
// and returns a count of newly persisted candidates plus whether the company
// already had any recruiter at confidence >= medium.
func (s *Service) FindForCompany(ctx context.Context, companyID string) (newCount int, hadStrong bool, err error) {
	c, err := s.GetCompany(ctx, companyID)
	if err != nil {
		return 0, false, err
	}
	// Step 1: existing recruiters at confidence != low → short-circuit.
	hadStrong, err = s.hasStrongRecruiter(ctx, companyID)
	if err != nil {
		return 0, false, err
	}
	if hadStrong {
		return 0, true, nil
	}
	var all []Candidate
	if s.mailto != nil && c.CareersURL != "" && c.Domain != "" {
		mail, err := s.mailto.Extract(ctx, c.CareersURL, c.Domain)
		if err == nil {
			all = append(all, mail...)
		}
	}
	if s.github != nil && c.GitHubOrg != "" {
		gh, err := s.github.Mine(ctx, c.GitHubOrg, c.Domain)
		if err == nil {
			all = append(all, gh...)
		}
	}
	n, err := s.PersistCandidates(ctx, companyID, all)
	return n, false, err
}

func (s *Service) hasStrongRecruiter(ctx context.Context, companyID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM recruiters
			WHERE company_id = $1::uuid AND confidence IN ('high','medium') AND NOT is_blocked
		)
	`, companyID).Scan(&ok)
	return ok, err
}

// HasAnyRecruiter returns true if the company has at least one recruiter (any
// confidence) that is not blocked.
func (s *Service) HasAnyRecruiter(ctx context.Context, companyID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM recruiters WHERE company_id = $1::uuid AND NOT is_blocked
		)
	`, companyID).Scan(&ok)
	return ok, err
}

// PersistCandidates filters + writes candidates into the recruiters table.
// Returns the number of newly inserted rows. Existing rows are left untouched.
func (s *Service) PersistCandidates(ctx context.Context, companyID string, cands []Candidate) (int, error) {
	inserted := 0
	for _, c := range cands {
		email := NormalizeEmail(c.Email)
		if email == "" {
			continue
		}
		if IsBlockedRoleAlias(email) {
			continue
		}
		blocked, err := s.IsDenylisted(ctx, email)
		if err != nil {
			return inserted, err
		}
		if blocked {
			continue
		}
		conf := c.Confidence
		if conf == "" {
			conf = ConfidenceLow
		}
		src := c.Source
		if src == "" {
			src = SourceManual
		}
		ct, err := s.pool.Exec(ctx, `
			INSERT INTO recruiters (company_id, email, name, title, source, confidence, evidence_url)
			VALUES ($1::uuid, $2, NULLIF($3,''), NULLIF($4,''), $5, $6, NULLIF($7,''))
			ON CONFLICT (email, company_id) DO NOTHING
		`, companyID, email, c.Name, c.Title, src, conf, c.EvidenceURL)
		if err != nil {
			return inserted, fmt.Errorf("insert recruiter %s: %w", email, err)
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}
	return inserted, nil
}

func (s *Service) IsDenylisted(ctx context.Context, email string) (bool, error) {
	email = NormalizeEmail(email)
	if email == "" {
		return false, nil
	}
	var ok bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM email_denylist WHERE email = $1)`, email).Scan(&ok)
	return ok, err
}

func (s *Service) AddToDenylist(ctx context.Context, email, reason string) error {
	email = NormalizeEmail(email)
	if email == "" {
		return errors.New("invalid email")
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO email_denylist (email, reason) VALUES ($1, NULLIF($2,''))
		 ON CONFLICT (email) DO UPDATE SET reason = COALESCE(EXCLUDED.reason, email_denylist.reason)`,
		email, strings.TrimSpace(reason))
	return err
}

// UpsertCompanyContact stores or updates the careers_url/domain/github_org
// fields for an existing company. Used by the admin bulk-paste endpoint when
// the operator provides metadata alongside recruiters.
func (s *Service) UpsertCompanyContact(ctx context.Context, slug, careersURL, domain, githubOrg string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return "", errors.New("empty slug")
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO companies (name, slug, domain, careers_url, github_org)
		VALUES ($1, $1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''))
		ON CONFLICT (slug) DO UPDATE SET
			domain      = COALESCE(EXCLUDED.domain,      companies.domain),
			careers_url = COALESCE(EXCLUDED.careers_url, companies.careers_url),
			github_org  = COALESCE(EXCLUDED.github_org,  companies.github_org)
		RETURNING id::text
	`, slug, domain, careersURL, githubOrg).Scan(&id)
	return id, err
}

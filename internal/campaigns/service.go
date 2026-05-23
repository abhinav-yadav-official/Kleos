package campaigns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

var ErrNotFound = errors.New("campaign not found")

type CreateInput struct {
	Name     string
	ResumeID string
	SMTPID   string
}

func (s *Service) Create(ctx context.Context, userID string, in CreateInput) (Campaign, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Campaign{}, errors.New("name required")
	}
	if in.ResumeID == "" || in.SMTPID == "" {
		return Campaign{}, errors.New("resume_id and smtp_id required")
	}
	var owns bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM resumes WHERE id=$1::uuid AND user_id=$2::uuid)`,
		in.ResumeID, userID).Scan(&owns); err != nil {
		return Campaign{}, err
	}
	if !owns {
		return Campaign{}, errors.New("resume not found for user")
	}
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM smtp_credentials WHERE id=$1::uuid AND user_id=$2::uuid)`,
		in.SMTPID, userID).Scan(&owns); err != nil {
		return Campaign{}, err
	}
	if !owns {
		return Campaign{}, errors.New("smtp credential not found for user")
	}

	var c Campaign
	err := s.pool.QueryRow(ctx, `
		INSERT INTO campaigns (user_id, name, resume_id, smtp_id)
		VALUES ($1::uuid, $2, $3::uuid, $4::uuid)
		RETURNING id::text, user_id::text, name, status, resume_id::text, smtp_id::text, created_at, updated_at
	`, userID, name, in.ResumeID, in.SMTPID).Scan(
		&c.ID, &c.UserID, &c.Name, &c.Status, &c.ResumeID, &c.SMTPID, &c.CreatedAt, &c.UpdatedAt,
	)
	return c, err
}

func (s *Service) List(ctx context.Context, userID string) ([]WithCounts, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id::text, c.user_id::text, c.name, c.status, c.resume_id::text, c.smtp_id::text,
			c.created_at, c.updated_at,
			COALESCE(state_counts, '{}'::jsonb)
		FROM campaigns c
		LEFT JOIN LATERAL (
			SELECT jsonb_object_agg(state, cnt) AS state_counts
			FROM (
				SELECT state, count(*) AS cnt FROM campaign_matches
				WHERE campaign_id = c.id GROUP BY state
			) s
		) sc ON true
		WHERE c.user_id = $1::uuid
		ORDER BY c.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WithCounts
	for rows.Next() {
		var w WithCounts
		var counts map[string]int
		if err := rows.Scan(&w.ID, &w.UserID, &w.Name, &w.Status, &w.ResumeID, &w.SMTPID,
			&w.CreatedAt, &w.UpdatedAt, &counts); err != nil {
			return nil, err
		}
		if counts == nil {
			counts = map[string]int{}
		}
		w.MatchesByState = counts
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Service) Get(ctx context.Context, userID, id string) (WithCounts, error) {
	var w WithCounts
	var counts map[string]int
	err := s.pool.QueryRow(ctx, `
		SELECT c.id::text, c.user_id::text, c.name, c.status, c.resume_id::text, c.smtp_id::text,
			c.created_at, c.updated_at,
			COALESCE((SELECT jsonb_object_agg(state, cnt) FROM (
				SELECT state, count(*) AS cnt FROM campaign_matches
				WHERE campaign_id = c.id GROUP BY state
			) s), '{}'::jsonb)
		FROM campaigns c
		WHERE c.id = $1::uuid AND c.user_id = $2::uuid
	`, id, userID).Scan(&w.ID, &w.UserID, &w.Name, &w.Status, &w.ResumeID, &w.SMTPID,
		&w.CreatedAt, &w.UpdatedAt, &counts)
	if errors.Is(err, pgx.ErrNoRows) {
		return WithCounts{}, ErrNotFound
	}
	if err != nil {
		return WithCounts{}, err
	}
	if counts == nil {
		counts = map[string]int{}
	}
	w.MatchesByState = counts
	return w, nil
}

func (s *Service) ListMatches(ctx context.Context, userID, campaignID, state string, limit, offset int) ([]MatchRow, error) {
	var owns bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM campaigns WHERE id=$1::uuid AND user_id=$2::uuid)`,
		campaignID, userID).Scan(&owns); err != nil {
		return nil, err
	}
	if !owns {
		return nil, ErrNotFound
	}
	args := []any{campaignID, limit, offset}
	stateFilter := ""
	if state != "" {
		stateFilter = "AND m.state = $4"
		args = append(args, state)
	}
	q := fmt.Sprintf(`
		SELECT m.id::text, m.campaign_id::text, m.job_id::text, m.match_score, m.state, m.matched_at,
			j.title, j.url, COALESCE(j.location,''), j.remote, j.source,
			COALESCE(co.name, '')
		FROM campaign_matches m
		JOIN jobs j ON j.id = m.job_id
		LEFT JOIN companies co ON co.id = j.company_id
		WHERE m.campaign_id = $1::uuid %s
		ORDER BY m.match_score DESC, m.matched_at DESC
		LIMIT $2 OFFSET $3
	`, stateFilter)
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MatchRow
	for rows.Next() {
		var r MatchRow
		if err := rows.Scan(&r.ID, &r.CampaignID, &r.JobID, &r.MatchScore, &r.State, &r.MatchedAt,
			&r.JobTitle, &r.JobURL, &r.JobLocation, &r.JobRemote, &r.JobSource, &r.CompanyName); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Service) SetStatus(ctx context.Context, userID, id, status string) (Campaign, error) {
	if !ValidStatus(status) {
		return Campaign{}, fmt.Errorf("invalid status: %s", status)
	}
	var c Campaign
	err := s.pool.QueryRow(ctx, `
		UPDATE campaigns SET status = $3, updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING id::text, user_id::text, name, status, resume_id::text, smtp_id::text, created_at, updated_at
	`, id, userID, status).Scan(&c.ID, &c.UserID, &c.Name, &c.Status, &c.ResumeID, &c.SMTPID, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Campaign{}, ErrNotFound
	}
	return c, err
}

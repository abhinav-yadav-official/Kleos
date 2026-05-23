package contentgen

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
	gen  Generator
}

func NewService(pool *pgxpool.Pool, gen Generator) *Service {
	return &Service{pool: pool, gen: gen}
}

var (
	ErrMatchNotFound    = errors.New("match not found")
	ErrNoRecruiter      = errors.New("no recruiter available for company")
	ErrContentQuality   = errors.New("content_quality")
)

// MatchContext is the bundle the prompt + persistence layer needs.
type MatchContext struct {
	MatchID       string
	CampaignID    string
	UserID        string
	JobTitle      string
	JobDesc       string
	CompanyName   string
	ResumeText    string
	TonePreset    string
	ToneAddendum  string
	RecruiterID   string
	RecruiterName string
}

func (s *Service) LoadMatchContext(ctx context.Context, matchID string) (MatchContext, error) {
	var c MatchContext
	err := s.pool.QueryRow(ctx, `
		SELECT m.id::text, m.campaign_id::text, ca.user_id::text,
			j.title, j.description,
			COALESCE(co.name, ''),
			r.parsed_text,
			COALESCE(p.tone_preset, 'warm'),
			COALESCE(p.tone_addendum, '')
		FROM campaign_matches m
		JOIN campaigns ca ON ca.id = m.campaign_id
		JOIN jobs j ON j.id = m.job_id
		LEFT JOIN companies co ON co.id = j.company_id
		JOIN resumes r ON r.id = ca.resume_id
		LEFT JOIN preferences p ON p.user_id = ca.user_id
		WHERE m.id = $1::uuid
	`, matchID).Scan(&c.MatchID, &c.CampaignID, &c.UserID, &c.JobTitle, &c.JobDesc,
		&c.CompanyName, &c.ResumeText, &c.TonePreset, &c.ToneAddendum)
	if errors.Is(err, pgx.ErrNoRows) {
		return MatchContext{}, ErrMatchNotFound
	}
	if err != nil {
		return MatchContext{}, err
	}
	// Best recruiter: highest confidence, not blocked, not denylisted.
	err = s.pool.QueryRow(ctx, `
		SELECT r.id::text, COALESCE(r.name, '')
		FROM recruiters r
		JOIN jobs j ON j.company_id = r.company_id
		JOIN campaign_matches m ON m.job_id = j.id
		WHERE m.id = $1::uuid
		  AND NOT r.is_blocked
		  AND NOT EXISTS (SELECT 1 FROM email_denylist d WHERE d.email = r.email)
		ORDER BY CASE r.confidence
			WHEN 'high' THEN 0
			WHEN 'medium' THEN 1
			WHEN 'low' THEN 2
			ELSE 3 END,
			r.created_at DESC
		LIMIT 1
	`, matchID).Scan(&c.RecruiterID, &c.RecruiterName)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNoRecruiter
	}
	return c, err
}

// GenerateOne builds the prompt for matchID, invokes the generator, scores the
// variants, persists 3 email_drafts rows (one marked chosen), and returns the
// final state ("generated" or "failed") with the chosen draft id when present.
func (s *Service) GenerateOne(ctx context.Context, matchID string) (state string, chosenDraftID string, err error) {
	mc, err := s.LoadMatchContext(ctx, matchID)
	if err != nil {
		return "", "", err
	}

	prompt, err := RenderPrompt(PromptContext{
		ToneInstruction: ToneInstructionFor(mc.TonePreset),
		UserAddendum:    mc.ToneAddendum,
		ResumeText:      mc.ResumeText,
		JobTitle:        mc.JobTitle,
		CompanyName:     mc.CompanyName,
		JobDescription:  mc.JobDesc,
		RecruiterName:   mc.RecruiterName,
	})
	if err != nil {
		return "", "", fmt.Errorf("render prompt: %w", err)
	}

	res, err := s.gen.Generate(ctx, prompt)
	if err != nil {
		return "failed", "", err
	}
	if len(res.Variants) != 3 {
		return "failed", "", fmt.Errorf("expected 3 variants, got %d", len(res.Variants))
	}

	scores := ScoreAll(res.Variants, mc.RecruiterName, mc.CompanyName)
	chosen := PickChosen(res.Variants, scores)

	if scores[chosen] > MaxSpamScore {
		return "failed", "", ErrContentQuality
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i, v := range res.Variants {
		var id string
		err := tx.QueryRow(ctx, `
			INSERT INTO email_drafts (match_id, recruiter_id, variant, subject, body_text, chosen, spam_score)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
			RETURNING id::text
		`, mc.MatchID, mc.RecruiterID, i+1, v.Subject, v.Body, i == chosen, scores[i]).Scan(&id)
		if err != nil {
			return "", "", fmt.Errorf("insert draft %d: %w", i+1, err)
		}
		if i == chosen {
			chosenDraftID = id
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return "generated", chosenDraftID, nil
}

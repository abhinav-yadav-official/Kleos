package preferences

import (
	"context"
	"errors"
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

func (s *Service) Get(ctx context.Context, userID string) (Record, error) {
	record := Record{}
	err := s.pool.QueryRow(ctx, `
		SELECT user_id::text, job_titles, job_functions, experience_level, locations,
			keywords_include, keywords_exclude, remote_only, tone_preset, tone_addendum, updated_at
		FROM preferences
		WHERE user_id = $1
	`, userID).Scan(
		&record.UserID, &record.JobTitles, &record.JobFunctions, &record.ExperienceLevel, &record.Locations,
		&record.KeywordsInclude, &record.KeywordsExclude, &record.RemoteOnly, &record.TonePreset, &record.ToneAddendum, &record.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.Replace(ctx, userID, Default(userID))
	}
	return record, err
}

func (s *Service) Replace(ctx context.Context, userID string, input Record) (Record, error) {
	record, err := normalize(userID, input)
	if err != nil {
		return Record{}, err
	}
	err = s.pool.QueryRow(ctx, `
		INSERT INTO preferences (
			user_id, job_titles, job_functions, experience_level, locations,
			keywords_include, keywords_exclude, remote_only, tone_preset, tone_addendum, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		ON CONFLICT (user_id) DO UPDATE SET
			job_titles = EXCLUDED.job_titles,
			job_functions = EXCLUDED.job_functions,
			experience_level = EXCLUDED.experience_level,
			locations = EXCLUDED.locations,
			keywords_include = EXCLUDED.keywords_include,
			keywords_exclude = EXCLUDED.keywords_exclude,
			remote_only = EXCLUDED.remote_only,
			tone_preset = EXCLUDED.tone_preset,
			tone_addendum = EXCLUDED.tone_addendum,
			updated_at = now()
		RETURNING user_id::text, job_titles, job_functions, experience_level, locations,
			keywords_include, keywords_exclude, remote_only, tone_preset, tone_addendum, updated_at
	`, record.UserID, record.JobTitles, record.JobFunctions, record.ExperienceLevel, record.Locations,
		record.KeywordsInclude, record.KeywordsExclude, record.RemoteOnly, record.TonePreset, record.ToneAddendum).Scan(
		&record.UserID, &record.JobTitles, &record.JobFunctions, &record.ExperienceLevel, &record.Locations,
		&record.KeywordsInclude, &record.KeywordsExclude, &record.RemoteOnly, &record.TonePreset, &record.ToneAddendum, &record.UpdatedAt,
	)
	return record, err
}

func normalize(userID string, input Record) (Record, error) {
	input.UserID = userID
	input.JobTitles = cleanList(input.JobTitles)
	input.JobFunctions = cleanList(input.JobFunctions)
	input.Locations = cleanList(input.Locations)
	input.KeywordsInclude = cleanList(input.KeywordsInclude)
	input.KeywordsExclude = cleanList(input.KeywordsExclude)
	input.ExperienceLevel = strings.TrimSpace(input.ExperienceLevel)
	if input.ExperienceLevel == "" {
		input.ExperienceLevel = "mid"
	}
	if !allowedExperienceLevel(input.ExperienceLevel) {
		return Record{}, errors.New("invalid experience level")
	}
	input.TonePreset = strings.TrimSpace(input.TonePreset)
	if input.TonePreset == "" {
		input.TonePreset = "warm"
	}
	if !allowedTonePreset(input.TonePreset) {
		return Record{}, errors.New("invalid tone preset")
	}
	input.ToneAddendum = strings.TrimSpace(input.ToneAddendum)
	if len(input.ToneAddendum) > 500 {
		return Record{}, errors.New("tone addendum must be at most 500 characters")
	}
	return input, nil
}

func cleanList(values []string) []string {
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func allowedExperienceLevel(value string) bool {
	switch value {
	case "entry", "mid", "senior", "staff", "principal":
		return true
	default:
		return false
	}
}

func allowedTonePreset(value string) bool {
	switch value {
	case "formal", "casual", "technical", "warm":
		return true
	default:
		return false
	}
}

package campaigns

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MatchThreshold is the minimum score required to insert a campaign_matches row.
const MatchThreshold = 0.4

// Tick walks every active campaign, scores unmatched jobs against the user's
// preferences (formula per plan §8), and inserts campaign_matches rows for
// jobs at or above MatchThreshold. Returns the number of new match rows.
func Tick(ctx context.Context, pool *pgxpool.Pool) (int, error) {
	type campRow struct {
		ID     string
		UserID string
	}
	rows, err := pool.Query(ctx,
		`SELECT id::text, user_id::text FROM campaigns WHERE status = 'active'`)
	if err != nil {
		return 0, err
	}
	var camps []campRow
	for rows.Next() {
		var c campRow
		if err := rows.Scan(&c.ID, &c.UserID); err != nil {
			rows.Close()
			return 0, err
		}
		camps = append(camps, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	type prefs struct {
		Titles, Functions, Locations, KwInclude, KwExclude []string
		RemoteOnly                                         bool
	}

	inserted := 0
	for _, c := range camps {
		var p prefs
		err := pool.QueryRow(ctx, `
			SELECT job_titles, job_functions, locations,
				keywords_include, keywords_exclude, remote_only
			FROM preferences WHERE user_id = $1::uuid
		`, c.UserID).Scan(&p.Titles, &p.Functions, &p.Locations, &p.KwInclude, &p.KwExclude, &p.RemoteOnly)
		if err != nil {
			return inserted, err
		}

		jobRows, err := pool.Query(ctx, `
			SELECT j.id::text, j.title, COALESCE(j.location,''), j.remote, j.description
			FROM jobs j
			WHERE NOT EXISTS (
				SELECT 1 FROM campaign_matches m
				WHERE m.campaign_id = $1::uuid AND m.job_id = j.id
			)
		`, c.ID)
		if err != nil {
			return inserted, err
		}
		type jobRow struct {
			ID, Title, Location, Description string
			Remote                            bool
		}
		var jobs []jobRow
		for jobRows.Next() {
			var j jobRow
			if err := jobRows.Scan(&j.ID, &j.Title, &j.Location, &j.Remote, &j.Description); err != nil {
				jobRows.Close()
				return inserted, err
			}
			jobs = append(jobs, j)
		}
		jobRows.Close()
		if err := jobRows.Err(); err != nil {
			return inserted, err
		}

		for _, j := range jobs {
			score := score(p.Titles, p.Functions, p.Locations, p.KwInclude, p.KwExclude,
				p.RemoteOnly, j.Title, j.Location, j.Remote, j.Description)
			if score < MatchThreshold {
				continue
			}
			ct, err := pool.Exec(ctx, `
				INSERT INTO campaign_matches (campaign_id, job_id, match_score, state)
				VALUES ($1::uuid, $2::uuid, $3, 'new')
				ON CONFLICT (campaign_id, job_id) DO NOTHING
			`, c.ID, j.ID, score)
			if err != nil {
				return inserted, err
			}
			if ct.RowsAffected() > 0 {
				inserted++
			}
		}
	}
	return inserted, nil
}

func score(titles, functions, locations, kwInc, kwExc []string, remoteOnly bool,
	jobTitle, jobLoc string, jobRemote bool, jobDesc string) float64 {
	titleLower := strings.ToLower(jobTitle)
	locLower := strings.ToLower(jobLoc)
	descLower := strings.ToLower(jobDesc)

	s := 0.0
	if anySubstring(titleLower, titles) {
		s += 0.4
	}
	if anySubstring(titleLower, functions) || anySubstring(descLower, functions) {
		s += 0.2
	}
	if jobRemote || anySubstring(locLower, locations) {
		s += 0.2
	}
	// Keywords include: +0.1 each, capped at +0.2.
	kwHits := 0
	for _, kw := range kwInc {
		if kw == "" {
			continue
		}
		if strings.Contains(descLower, strings.ToLower(kw)) {
			kwHits++
		}
	}
	if kwHits > 0 {
		bump := float64(kwHits) * 0.1
		if bump > 0.2 {
			bump = 0.2
		}
		s += bump
	}
	for _, kw := range kwExc {
		if kw == "" {
			continue
		}
		if strings.Contains(descLower, strings.ToLower(kw)) {
			s -= 0.5
			break
		}
	}
	if remoteOnly && !jobRemote {
		// Filter out non-remote jobs when user asked remote-only.
		return 0
	}
	return s
}

func anySubstring(haystack string, needles []string) bool {
	for _, n := range needles {
		if n == "" {
			continue
		}
		if strings.Contains(haystack, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

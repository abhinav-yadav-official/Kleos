package main

import (
	"context"

	apphttp "github.com/abhinav-yadav-official/Kleos/internal/http"
	"github.com/jackc/pgx/v5/pgxpool"
)

type poolAdapter struct {
	pool *pgxpool.Pool
}

func newPoolAdapter(pool *pgxpool.Pool) *poolAdapter {
	return &poolAdapter{pool: pool}
}

// ListPool returns recruiters in the shared pool whose company.country matches
// the requested ISO code. Denylisted and blocked recruiters are filtered out.
// Ordered by highest confidence first, newest first.
func (a *poolAdapter) ListPool(ctx context.Context, country string, limit, offset int) ([]apphttp.PoolEntry, error) {
	rows, err := a.pool.Query(ctx, `
		SELECT r.id::text, r.email::text, COALESCE(r.name, ''), COALESCE(r.title, ''),
			r.confidence, r.source,
			c.slug, COALESCE(c.name, ''), COALESCE(c.domain, ''), COALESCE(c.country, ''),
			r.created_at
		FROM recruiters r
		JOIN companies c ON c.id = r.company_id
		WHERE c.country = $1
		  AND NOT r.is_blocked
		  AND NOT EXISTS (SELECT 1 FROM email_denylist d WHERE d.email = r.email)
		ORDER BY CASE r.confidence
		    WHEN 'high' THEN 0
		    WHEN 'medium' THEN 1
		    WHEN 'low' THEN 2
		    ELSE 3 END,
		    r.created_at DESC
		LIMIT $2 OFFSET $3
	`, country, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []apphttp.PoolEntry{}
	for rows.Next() {
		var e apphttp.PoolEntry
		if err := rows.Scan(&e.RecruiterID, &e.Email, &e.Name, &e.Title, &e.Confidence, &e.Source,
			&e.CompanySlug, &e.CompanyName, &e.CompanyDomain, &e.CompanyCountry, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

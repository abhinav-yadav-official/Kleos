package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/almostturingcomplete/Kleos/internal/config"
	"github.com/almostturingcomplete/Kleos/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

type csvRow struct {
	ContactEmail string
	Title        string
	FirstName    string
	LastName     string
	JobTitle     string
	CompanyName  string
	Phone        string
	Mobile       string
	CompanyType  string
	CompanySize  string
	CompanyLoc   string
}

type csvRecord struct {
	row  csvRow
	line int
}

type companyGroup struct {
	records []csvRecord
}

func main() {
	csvPath := os.Getenv("CSV_PATH")
	if csvPath == "" {
		slog.Error("CSV_PATH env var is required")
		os.Exit(1)
	}

	cfg := config.Load()
	ctx := context.Background()

	pg, err := db.ConnectPostgres(ctx, cfg.DBDSN)
	if err != nil {
		slog.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer pg.Close()

	f, err := os.Open(csvPath)
	if err != nil {
		slog.Error("open csv", "error", err)
		os.Exit(1)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	// skip comment lines and empty lines before header
	var header []string
	for {
		peek, err := r.Read()
		if err != nil {
			slog.Error("read csv header", "error", err)
			os.Exit(1)
		}
		if len(peek) == 1 && len(peek[0]) > 0 && peek[0][0] == '#' {
			continue
		}
		if len(peek) == 1 && peek[0] == "" {
			continue
		}
		header = peek
		break
	}
	if len(header) < 3 {
		slog.Error("unexpected header", "header", header)
		os.Exit(1)
	}
	col := buildColMap(header)

	groups := make(map[string]*companyGroup)
	var lineOrder []string

	for line := 2; ; line++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("skip row", "line", line, "error", err)
			continue
		}
		row := csvRow{
			ContactEmail: colValMulti(rec, col, []string{"Contact Email", "Email"}),
			Title:        colValMulti(rec, col, []string{"Title"}),
			FirstName:    colValMulti(rec, col, []string{"First Name", "Name"}),
			LastName:     colValMulti(rec, col, []string{"Last Name"}),
			JobTitle:     colValMulti(rec, col, []string{"Job Title", "Title"}),
			CompanyName:  strings.TrimSpace(colValMulti(rec, col, []string{"Company Name", "Company"})),
			Phone:        colValMulti(rec, col, []string{"Phone", "Mobile"}),
			Mobile:       colValMulti(rec, col, []string{"Mobile", "Phone"}),
			CompanyType:  colVal(rec, col, "Company Type"),
			CompanySize:  colVal(rec, col, "Company Size"),
			CompanyLoc:   colVal(rec, col, "Company Location"),
		}
		if row.ContactEmail == "" || row.CompanyName == "" {
			continue
		}

		g, ok := groups[row.CompanyName]
		if !ok {
			g = &companyGroup{}
			groups[row.CompanyName] = g
			lineOrder = append(lineOrder, row.CompanyName)
		}
		g.records = append(g.records, csvRecord{row: row, line: line})
	}

	slog.Info("parsed csv", "companies", len(groups), "total_rows", countRows(groups))

	for _, companyName := range lineOrder {
		g := groups[companyName]
		if err := importCompany(ctx, pg.Pool(), companyName, g); err != nil {
			slog.Error("import company", "name", companyName, "error", err)
			continue
		}
	}

	slog.Info("bulk import complete")
}

func importCompany(ctx context.Context, pool *pgxpool.Pool, companyName string, g *companyGroup) error {
	companyName = strings.TrimSpace(companyName)
	if companyName == "" {
		return nil
	}

	slug := slugify(companyName)
	first := g.records[0].row

	companyID, err := upsertCompany(ctx, pool, slug, companyName, first.CompanyType, first.CompanySize, first.CompanyLoc)
	if err != nil {
		return fmt.Errorf("upsert company: %w", err)
	}

	inserted := 0
	for _, r := range g.records {
		email := normalizeEmail(r.row.ContactEmail)
		if email == "" {
			continue
		}
		name := strings.TrimSpace(r.row.FirstName + " " + r.row.LastName)
		title := r.row.JobTitle
		phone := r.row.Phone
		if phone == "" {
			phone = r.row.Mobile
		}

		ct, err := pool.Exec(ctx, `
			INSERT INTO recruiters (company_id, email, name, title, source, confidence, phone, mobile)
			VALUES ($1::uuid, $2, NULLIF($3,''), NULLIF($4,''), 'csv_import', 'high', NULLIF($5,''), NULLIF($6,''))
			ON CONFLICT (email, company_id) DO NOTHING
		`, companyID, email, name, title, phone, r.row.Mobile)
		if err != nil {
			slog.Warn("insert recruiter", "email", email, "error", err)
			continue
		}
		if ct.RowsAffected() > 0 {
			inserted++
		}
	}

	slog.Info("imported company", "name", companyName, "slug", slug, "recruiters", inserted, "total_rows", len(g.records))
	return nil
}

func upsertCompany(ctx context.Context, pool *pgxpool.Pool, slug, name, companyType, companySize, companyLoc string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO companies (name, slug, country, company_type, company_size, company_location)
		VALUES ($1, $2, 'IN', NULLIF($3,''), NULLIF($4,''), NULLIF($5,''))
		ON CONFLICT (slug) DO UPDATE SET
			name           = COALESCE(NULLIF(EXCLUDED.name, ''), companies.name),
			company_type   = COALESCE(NULLIF(EXCLUDED.company_type, ''), companies.company_type),
			company_size   = COALESCE(NULLIF(EXCLUDED.company_size, ''), companies.company_size),
			company_location = COALESCE(NULLIF(EXCLUDED.company_location, ''), companies.company_location)
		RETURNING id::text
	`, name, slug, companyType, companySize, companyLoc).Scan(&id)
	return id, err
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else if r == ' ' || r == '-' || r == '_' || r == '.' {
			b.WriteRune('-')
		}
	}
	slug := b.String()
	slug = strings.Trim(slug, "-")
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	if slug == "" {
		slug = "company"
	}
	return slug
}

func normalizeEmail(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.Contains(s, "@") || strings.Count(s, "@") != 1 {
		return ""
	}
	return s
}

func buildColMap(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		h = strings.TrimSpace(h)
		m[h] = i
	}
	return m
}

func colVal(row []string, col map[string]int, name string) string {
	idx, ok := col[name]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func colValMulti(row []string, col map[string]int, names []string) string {
	for _, name := range names {
		if v := colVal(row, col, name); v != "" {
			return v
		}
	}
	return ""
}

func countRows(groups map[string]*companyGroup) int {
	n := 0
	for _, g := range groups {
		n += len(g.records)
	}
	return n
}

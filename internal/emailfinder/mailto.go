package emailfinder

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// MailtoExtractor walks a small set of well-known pages on the company site
// (careers, /about, /team, /contact) and harvests mailto: links whose domain
// matches the company's domain.
type MailtoExtractor struct {
	client *http.Client
}

func NewMailtoExtractor(client *http.Client) *MailtoExtractor {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &MailtoExtractor{client: client}
}

// extraPaths are appended to the careers URL host when probing for mailtos.
var extraPaths = []string{"", "/about", "/team", "/contact", "/jobs", "/careers"}

var mailtoRE = regexp.MustCompile(`(?i)mailto:([^"'\s>?]+)`)

// Extract fetches the careers page (and a few sibling pages) and returns
// matching Candidates. companyDomain is the bare domain (e.g. "acme.com")
// used to filter mailto matches.
func (m *MailtoExtractor) Extract(ctx context.Context, careersURL, companyDomain string) ([]Candidate, error) {
	if careersURL == "" {
		return nil, nil
	}
	base, err := url.Parse(careersURL)
	if err != nil {
		return nil, fmt.Errorf("parse careers_url: %w", err)
	}
	companyDomain = strings.ToLower(strings.TrimSpace(companyDomain))

	seen := map[string]Candidate{}
	for _, path := range extraPaths {
		page := *base
		if path != "" {
			page.Path = path
			page.RawQuery = ""
			page.Fragment = ""
		}
		c, err := m.fetchAndExtract(ctx, page.String(), companyDomain)
		if err != nil {
			slog.Debug("mailto fetch failed", "url", page.String(), "error", err)
			continue
		}
		for _, cand := range c {
			if _, ok := seen[cand.Email]; ok {
				continue
			}
			seen[cand.Email] = cand
		}
	}
	out := make([]Candidate, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	return out, nil
}

func (m *MailtoExtractor) fetchAndExtract(ctx context.Context, pageURL, companyDomain string) ([]Candidate, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "kleos-emailfinder/0.1 (+https://abhiyadav.in/kleos)")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	return extractMailtos(string(body), pageURL, companyDomain), nil
}

func extractMailtos(body, pageURL, companyDomain string) []Candidate {
	matches := mailtoRE.FindAllStringSubmatch(body, -1)
	out := make([]Candidate, 0, len(matches))
	seen := map[string]struct{}{}
	for _, m := range matches {
		raw := m[1]
		// strip any query string ?subject=...
		if i := strings.IndexByte(raw, '?'); i >= 0 {
			raw = raw[:i]
		}
		email := NormalizeEmail(raw)
		if email == "" {
			continue
		}
		if IsBlockedRoleAlias(email) {
			continue
		}
		// Domain match: company domain or subdomain of it.
		dom := domainPart(email)
		if companyDomain != "" && dom != companyDomain && !strings.HasSuffix(dom, "."+companyDomain) {
			continue
		}
		if _, ok := seen[email]; ok {
			continue
		}
		seen[email] = struct{}{}

		conf := ConfidenceMedium
		if IsRecruitingRoleAlias(email) {
			conf = ConfidenceHigh
		}
		out = append(out, Candidate{
			Email:       email,
			Source:      SourceMailto,
			Confidence:  conf,
			EvidenceURL: pageURL,
		})
	}
	return out
}

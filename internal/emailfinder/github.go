package emailfinder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultGitHubBase = "https://api.github.com"

// GitHubMiner walks an organization's most recently updated repositories,
// pulls a few pages of commits from each, and harvests author emails as
// low-confidence recruiter candidates. Plan §9 step 3.
type GitHubMiner struct {
	client   *http.Client
	baseURL  string
	token    string
	maxRepos int
	maxPages int
}

func NewGitHubMiner(client *http.Client, baseURL, token string) *GitHubMiner {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if baseURL == "" {
		baseURL = DefaultGitHubBase
	}
	return &GitHubMiner{
		client:   client,
		baseURL:  strings.TrimRight(baseURL, "/"),
		token:    token,
		maxRepos: 10,
		maxPages: 3,
	}
}

type ghRepo struct {
	Name string `json:"name"`
}

type ghCommit struct {
	Commit ghCommitInner `json:"commit"`
}

type ghCommitInner struct {
	Author ghAuthor `json:"author"`
}

type ghAuthor struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (g *GitHubMiner) Mine(ctx context.Context, org, companyDomain string) ([]Candidate, error) {
	if org == "" {
		return nil, nil
	}
	repos, err := g.listRepos(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("list repos: %w", err)
	}
	seen := map[string]Candidate{}
	for i, repo := range repos {
		if i >= g.maxRepos {
			break
		}
		for page := 1; page <= g.maxPages; page++ {
			commits, err := g.listCommits(ctx, org, repo.Name, page)
			if err != nil {
				break
			}
			if len(commits) == 0 {
				break
			}
			for _, c := range commits {
				email := NormalizeEmail(c.Commit.Author.Email)
				if email == "" {
					continue
				}
				if g.skipEmail(email, companyDomain) {
					continue
				}
				if _, ok := seen[email]; ok {
					continue
				}
				seen[email] = Candidate{
					Email:       email,
					Name:        c.Commit.Author.Name,
					Source:      SourceGitHub,
					Confidence:  ConfidenceLow,
					EvidenceURL: fmt.Sprintf("https://github.com/%s/%s", org, repo.Name),
				}
			}
		}
	}
	out := make([]Candidate, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	return out, nil
}

func (g *GitHubMiner) skipEmail(email, companyDomain string) bool {
	dom := domainPart(email)
	if dom == "" {
		return true
	}
	switch {
	case strings.HasSuffix(dom, "users.noreply.github.com"):
		return true
	case dom == "github.com":
		return true
	case strings.HasPrefix(localPart(email), "noreply"):
		return true
	}
	if IsBlockedRoleAlias(email) {
		return true
	}
	// If we know the company's verified domain, prefer matches; otherwise
	// keep the candidate but mark low (handled by caller).
	if companyDomain != "" && dom != companyDomain && !strings.HasSuffix(dom, "."+companyDomain) {
		// Skip personal addresses on other domains to limit noise.
		return true
	}
	return false
}

func (g *GitHubMiner) listRepos(ctx context.Context, org string) ([]ghRepo, error) {
	url := fmt.Sprintf("%s/orgs/%s/repos?per_page=100&sort=updated", g.baseURL, org)
	var repos []ghRepo
	if err := g.getJSON(ctx, url, &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func (g *GitHubMiner) listCommits(ctx context.Context, org, repo string, page int) ([]ghCommit, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=100&page=%d", g.baseURL, org, repo, page)
	var commits []ghCommit
	if err := g.getJSON(ctx, url, &commits); err != nil {
		return nil, err
	}
	return commits, nil
}

func (g *GitHubMiner) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "kleos-emailfinder/0.1")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d: %s", resp.StatusCode, truncate(string(body), 200))
	}
	return json.Unmarshal(body, out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

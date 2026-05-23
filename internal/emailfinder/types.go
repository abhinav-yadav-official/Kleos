package emailfinder

import "strings"

const (
	SourceManual = "manual"
	SourceMailto = "mailto"
	SourceGitHub = "github"

	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

type Candidate struct {
	Email       string
	Name        string
	Title       string
	Source      string
	Confidence  string
	EvidenceURL string
}

// RolePrefixes are role-alias localparts that should map to confidence=high
// when found at the company domain.
var RolePrefixes = []string{
	"jobs", "careers", "talent", "recruit", "recruiting", "recruiters",
	"hiring", "hr", "people", "team",
}

// Blocked role aliases that should never be contacted regardless of source.
var BlockedRolePrefixes = []string{
	"security", "abuse", "noreply", "no-reply", "postmaster", "webmaster",
	"admin", "root", "sales", "support", "info", "contact", "help",
	"privacy", "legal", "compliance", "press", "media", "billing", "accounts",
}

// IsBlockedRoleAlias returns true when the email's localpart is on the
// BlockedRolePrefixes list — these are filtered before any persistence.
func IsBlockedRoleAlias(email string) bool {
	local := strings.ToLower(localPart(email))
	for _, p := range BlockedRolePrefixes {
		if local == p {
			return true
		}
	}
	return false
}

// IsRecruitingRoleAlias returns true when the email's localpart matches a
// recruiting role prefix (e.g. jobs@, careers@).
func IsRecruitingRoleAlias(email string) bool {
	local := strings.ToLower(localPart(email))
	for _, p := range RolePrefixes {
		if local == p {
			return true
		}
	}
	return false
}

func localPart(email string) string {
	at := strings.IndexByte(email, '@')
	if at < 0 {
		return email
	}
	return email[:at]
}

func domainPart(email string) string {
	at := strings.IndexByte(email, '@')
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[at+1:])
}

// NormalizeEmail lowercases the address and trims surrounding whitespace.
// Returns empty string if the input is not a plausible email.
func NormalizeEmail(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.Contains(s, "@") || strings.Count(s, "@") != 1 {
		return ""
	}
	local := localPart(s)
	domain := domainPart(s)
	if local == "" || domain == "" || !strings.Contains(domain, ".") {
		return ""
	}
	return s
}

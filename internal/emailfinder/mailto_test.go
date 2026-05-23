package emailfinder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

const careersHTML = `<html><body>
<a href="mailto:jobs@acme.com?subject=hi">Jobs</a>
<a href="mailto:talent@acme.com">Talent</a>
<a href="mailto:jane.doe@acme.com">Jane</a>
<a href="mailto:security@acme.com">Security</a>
<a href="mailto:noreply@acme.com">No Reply</a>
<a href="mailto:hr@otherco.com">Other</a>
<a href="mailto:JANE.DOE@acme.com">Jane Again</a>
</body></html>`

func TestMailtoExtract(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/careers" || r.URL.Path == "/" {
			_, _ = w.Write([]byte(careersHTML))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	ex := NewMailtoExtractor(srv.Client())
	got, err := ex.Extract(context.Background(), srv.URL+"/careers", "acme.com")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	emails := make([]string, 0, len(got))
	confByEmail := map[string]string{}
	for _, c := range got {
		emails = append(emails, c.Email)
		confByEmail[c.Email] = c.Confidence
	}
	sort.Strings(emails)

	want := []string{"jane.doe@acme.com", "jobs@acme.com", "talent@acme.com"}
	if len(emails) != len(want) {
		t.Fatalf("emails = %v, want %v", emails, want)
	}
	for i, w := range want {
		if emails[i] != w {
			t.Errorf("emails[%d] = %q, want %q", i, emails[i], w)
		}
	}

	if confByEmail["jobs@acme.com"] != ConfidenceHigh {
		t.Errorf("jobs@ should be high, got %q", confByEmail["jobs@acme.com"])
	}
	if confByEmail["talent@acme.com"] != ConfidenceHigh {
		t.Errorf("talent@ should be high, got %q", confByEmail["talent@acme.com"])
	}
	if confByEmail["jane.doe@acme.com"] != ConfidenceMedium {
		t.Errorf("jane.doe@ should be medium, got %q", confByEmail["jane.doe@acme.com"])
	}
}

func TestExtractMailtosFiltersBlockedAndForeign(t *testing.T) {
	out := extractMailtos(careersHTML, "https://acme.com/careers", "acme.com")
	for _, c := range out {
		if c.Email == "security@acme.com" || c.Email == "noreply@acme.com" {
			t.Errorf("blocked role %q leaked through", c.Email)
		}
		if c.Email == "hr@otherco.com" {
			t.Errorf("foreign-domain %q leaked through", c.Email)
		}
	}
}

func TestRolePrefixHelpers(t *testing.T) {
	if !IsBlockedRoleAlias("security@acme.com") {
		t.Error("security@ should be blocked")
	}
	if !IsRecruitingRoleAlias("jobs@acme.com") {
		t.Error("jobs@ should be recruiting")
	}
	if IsBlockedRoleAlias("jane@acme.com") {
		t.Error("personal address misflagged")
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"  Jane@Acme.COM ": "jane@acme.com",
		"jane@acme":        "",
		"jane":             "",
		"":                 "",
		"a@b@c.com":        "",
	}
	for in, want := range cases {
		if got := NormalizeEmail(in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

package emailfinder

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubMineFiltersAndAggregates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/orgs/acme/repos":
			fmt.Fprintln(w, `[{"name":"core"},{"name":"docs"}]`)
		case strings.HasPrefix(r.URL.Path, "/repos/acme/core/commits"):
			if r.URL.Query().Get("page") == "1" {
				fmt.Fprintln(w, `[
					{"commit":{"author":{"name":"Alice","email":"alice@acme.com"}}},
					{"commit":{"author":{"name":"Bot","email":"123456+bot@users.noreply.github.com"}}},
					{"commit":{"author":{"name":"Sec","email":"security@acme.com"}}},
					{"commit":{"author":{"name":"Bob","email":"bob@otherco.com"}}}
				]`)
				return
			}
			fmt.Fprintln(w, `[]`)
		case strings.HasPrefix(r.URL.Path, "/repos/acme/docs/commits"):
			if r.URL.Query().Get("page") == "1" {
				fmt.Fprintln(w, `[
					{"commit":{"author":{"name":"Alice","email":"alice@acme.com"}}},
					{"commit":{"author":{"name":"Carol","email":"carol@acme.com"}}}
				]`)
				return
			}
			fmt.Fprintln(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	gm := NewGitHubMiner(srv.Client(), srv.URL, "")
	got, err := gm.Mine(context.Background(), "acme", "acme.com")
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	byEmail := map[string]Candidate{}
	for _, c := range got {
		byEmail[c.Email] = c
	}
	if _, ok := byEmail["alice@acme.com"]; !ok {
		t.Error("alice missing")
	}
	if _, ok := byEmail["carol@acme.com"]; !ok {
		t.Error("carol missing")
	}
	if _, ok := byEmail["security@acme.com"]; ok {
		t.Error("security role alias leaked")
	}
	if _, ok := byEmail["123456+bot@users.noreply.github.com"]; ok {
		t.Error("github noreply leaked")
	}
	if _, ok := byEmail["bob@otherco.com"]; ok {
		t.Error("foreign-domain address leaked")
	}
	for _, c := range got {
		if c.Confidence != ConfidenceLow {
			t.Errorf("%s confidence = %s, want low", c.Email, c.Confidence)
		}
		if c.Source != SourceGitHub {
			t.Errorf("%s source = %s", c.Email, c.Source)
		}
	}
}

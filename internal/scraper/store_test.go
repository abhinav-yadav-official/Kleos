package scraper

import "testing"

func TestNormalizeSlug(t *testing.T) {
	cases := map[string]string{
		"Acme":          "acme",
		"  Acme Co  ":   "acme-co",
		"Foo/Bar_Baz":   "foo-bar-baz",
		"--hello--":     "hello",
		"Hello, World!": "hello-world",
	}
	for in, want := range cases {
		if got := NormalizeSlug(in); got != want {
			t.Errorf("NormalizeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

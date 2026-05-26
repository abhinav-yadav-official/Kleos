package apphttp

import "testing"

func TestGoogleStateRoundTrip(t *testing.T) {
	state, cookie, err := newGoogleOAuthState("/dashboard")
	if err != nil {
		t.Fatalf("new state: %v", err)
	}
	next, ok := parseGoogleOAuthState(state, cookie)
	if !ok {
		t.Fatal("parse should succeed for matching state")
	}
	if next != "/dashboard" {
		t.Fatalf("next = %q", next)
	}
}

func TestGoogleStateRejectsMismatch(t *testing.T) {
	_, cookie, err := newGoogleOAuthState("/dashboard")
	if err != nil {
		t.Fatalf("new state: %v", err)
	}
	if _, ok := parseGoogleOAuthState("tampered", cookie); ok {
		t.Fatal("expected reject for mismatched state")
	}
}

func TestSafeNextRejectsExternal(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
	}{
		{"/dashboard", "/dashboard"},
		{"https://evil.com", ""},
		{"//evil.com/path", ""},
		{"", ""},
		{"  ", ""},
		{"javascript:alert(1)", ""},
	} {
		got := safeNext(c.in)
		if got != c.want {
			t.Errorf("safeNext(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

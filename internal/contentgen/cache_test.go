package contentgen

import "testing"

func TestResumeHashStable(t *testing.T) {
	a := ResumeHash("hello world")
	b := ResumeHash("hello world")
	if a != b {
		t.Fatalf("hash mismatch for identical input: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("expected 64-char hex sha256, got %d", len(a))
	}
}

func TestResumeHashDistinct(t *testing.T) {
	if ResumeHash("a") == ResumeHash("b") {
		t.Fatal("distinct inputs must hash differently")
	}
}

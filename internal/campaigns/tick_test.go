package campaigns

import "testing"

func TestScoreTitleAndRemote(t *testing.T) {
	got := score(
		[]string{"backend engineer"},
		[]string{"platform"},
		[]string{"remote"},
		[]string{"go", "postgres"},
		[]string{"php"},
		false,
		"Senior Backend Engineer",
		"Remote - US",
		true,
		"We use go and postgres on the platform team.",
	)
	// 0.4 title + 0.2 function (description) + 0.2 remote + 0.2 kw cap = 1.0
	if got < 0.99 || got > 1.01 {
		t.Fatalf("score = %v, want 1.0", got)
	}
}

func TestScoreExcludedKeyword(t *testing.T) {
	got := score(
		[]string{"engineer"},
		nil,
		[]string{"new york"},
		nil,
		[]string{"php"},
		false,
		"Senior Engineer",
		"New York, NY",
		false,
		"We are a PHP shop.",
	)
	// 0.4 title + 0.2 loc - 0.5 = 0.1 (below threshold)
	if got > 0.11 || got < 0.09 {
		t.Fatalf("score = %v, want ~0.1", got)
	}
	if got >= MatchThreshold {
		t.Fatalf("score %v should not exceed threshold", got)
	}
}

func TestScoreRemoteOnlyFiltersNonRemote(t *testing.T) {
	got := score(
		[]string{"engineer"}, nil, nil, nil, nil,
		true,
		"Senior Engineer", "New York", false, "Anything",
	)
	if got != 0 {
		t.Fatalf("remote_only should zero non-remote jobs, got %v", got)
	}
}

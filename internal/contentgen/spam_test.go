package contentgen

import "testing"

const cleanBody = `Saw your platform team's migration to Postgres 16 with logical replication is mature and worth a closer look from anyone working on similar systems today.
I shipped a similar cutover at a 30 TB shop with zero downtime over six months while keeping the read replicas perfectly in sync.
Before that I owned the search infrastructure at a mid-stage company running close to four thousand queries per second across two regions.
I have been writing Go services on Postgres for the better part of eight years and would value a short call next week if that timing works on your side.

Best, Jane`

func TestScoreVariantClean(t *testing.T) {
	v := Variant{Subject: "Engineer interested in your platform team", Body: cleanBody}
	got := ScoreVariant(v, "Alex", "Acme")
	if got > 0.05 {
		t.Fatalf("clean variant score = %v, want near 0", got)
	}
}

func TestScoreVariantBannedWord(t *testing.T) {
	v := Variant{Subject: "Free chat about engineering", Body: cleanBody}
	if ScoreVariant(v, "Alex", "Acme") < 0.15 {
		t.Fatalf("banned word not penalized")
	}
}

func TestScoreVariantLengthLink(t *testing.T) {
	v := Variant{
		Subject: "Hi",
		Body:    "See https://abhiyadav.in for details. Short!",
	}
	got := ScoreVariant(v, "Alex", "Acme")
	// subject too short (0.10) + body too short (0.10) + 1 bang (0.10) + link (0.20)
	if got < 0.49 || got > 0.55 {
		t.Fatalf("penalty score = %v, want ~0.5", got)
	}
}

func TestScoreAllAddsSamenessPenalty(t *testing.T) {
	v1 := Variant{Subject: "Engineer interested in your platform team", Body: cleanBody}
	v2 := Variant{Subject: "Engineer interested in your platform team", Body: cleanBody}
	v3 := Variant{Subject: "Backend engineer interested in your team", Body: cleanBody}
	scores := ScoreAll([]Variant{v1, v2, v3}, "Alex", "Acme")
	// v1 and v2 are identical; both should score above clean baseline.
	if scores[0] <= 0.05 || scores[1] <= 0.05 {
		t.Fatalf("expected sameness penalty: %v", scores)
	}
}

func TestPickChosenLowestScore(t *testing.T) {
	v := []Variant{
		{Body: "longer body version A"},
		{Body: "short B"},
		{Body: "long body version C"},
	}
	scores := []float64{0.20, 0.10, 0.10}
	if got := PickChosen(v, scores); got != 1 {
		t.Fatalf("picked %d, want 1 (lowest score, tie-broken by shortest)", got)
	}
}

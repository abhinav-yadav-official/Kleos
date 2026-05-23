package contentgen

import (
	"regexp"
	"strings"
)

// bannedWords is the case-insensitive blocklist from §10 HARD RULE 5.
var bannedWords = []string{
	"free", "guarantee", "urgent", "act now", "limited time", "click here",
	"winner", "congratulations", "risk-free", "no obligation", "cash", "bonus",
	"opportunity of a lifetime", "once in a lifetime", "dear friend",
	"dear sir or madam", "to whom it may concern", "hi dear",
	"amazing", "incredible", "unbeatable", "exclusive deal", "special promotion",
	"100%", "$$$", "!!!",
}

var (
	linkRE     = regexp.MustCompile(`https?://`)
	allCapsRE  = regexp.MustCompile(`\b[A-Z]{4,}\b`)
	wordSplit  = regexp.MustCompile(`\s+`)
	bangCount  = regexp.MustCompile(`!`)
	puncRE     = regexp.MustCompile(`[^\w\s]+`)
)

// ScoreVariant computes the spam_score for a single variant per §10. Lower is
// better. companyName and recruiterName drive the "no recruiter or company"
// penalty.
func ScoreVariant(v Variant, recruiterName, companyName string) float64 {
	subject := strings.TrimSpace(v.Subject)
	body := strings.TrimSpace(v.Body)
	combined := subject + "\n" + body
	lower := strings.ToLower(combined)

	score := 0.0
	for _, w := range bannedWords {
		if strings.Contains(lower, w) {
			score += 0.15
			break
		}
	}

	subjectWords := splitWords(subject)
	if len(subjectWords) > 9 || len(subjectWords) < 4 {
		score += 0.10
	}
	bodyWords := splitWords(body)
	if len(bodyWords) > 160 || len(bodyWords) < 70 {
		score += 0.10
	}

	if n := len(bangCount.FindAllString(combined, -1)); n > 0 {
		score += 0.10 * float64(n)
	}
	if n := len(allCapsRE.FindAllString(combined, -1)); n > 0 {
		score += 0.10 * float64(n)
	}
	if linkRE.MatchString(combined) {
		score += 0.20
	}
	if recruiterName == "" && (companyName == "" || !strings.Contains(lower, strings.ToLower(companyName))) {
		score += 0.10
	}
	return score
}

// ScoreAll computes per-variant scores plus a sameness penalty: +0.05 per
// repeated 4-gram across variants. Returns the per-variant scores in input
// order with sameness already added.
func ScoreAll(variants []Variant, recruiterName, companyName string) []float64 {
	out := make([]float64, len(variants))
	for i, v := range variants {
		out[i] = ScoreVariant(v, recruiterName, companyName)
	}
	if len(variants) < 2 {
		return out
	}
	grams := make([]map[string]struct{}, len(variants))
	for i, v := range variants {
		grams[i] = fourGrams(v.Subject + " " + v.Body)
	}
	// Count shared 4-grams across distinct pairs and split penalty equally.
	for i := 0; i < len(variants); i++ {
		for j := i + 1; j < len(variants); j++ {
			shared := 0
			for g := range grams[i] {
				if _, ok := grams[j][g]; ok {
					shared++
				}
			}
			if shared == 0 {
				continue
			}
			penalty := 0.05 * float64(shared)
			out[i] += penalty / 2
			out[j] += penalty / 2
		}
	}
	return out
}

// PickChosen returns the index of the lowest-scoring variant, tie-broken by
// shortest body length.
func PickChosen(variants []Variant, scores []float64) int {
	best := 0
	for i := 1; i < len(variants); i++ {
		switch {
		case scores[i] < scores[best]:
			best = i
		case scores[i] == scores[best] && len(variants[i].Body) < len(variants[best].Body):
			best = i
		}
	}
	return best
}

func splitWords(s string) []string {
	if s == "" {
		return nil
	}
	cleaned := puncRE.ReplaceAllString(s, " ")
	out := wordSplit.Split(strings.TrimSpace(cleaned), -1)
	if len(out) == 1 && out[0] == "" {
		return nil
	}
	return out
}

func fourGrams(s string) map[string]struct{} {
	words := splitWords(strings.ToLower(s))
	out := map[string]struct{}{}
	for i := 0; i+4 <= len(words); i++ {
		out[strings.Join(words[i:i+4], " ")] = struct{}{}
	}
	return out
}

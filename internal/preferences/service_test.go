package preferences

import (
	"strings"
	"testing"
)

func TestNormalizeTrimsListsAndDefaultsToneFields(t *testing.T) {
	record, err := normalize("user-1", Record{
		JobTitles:       []string{" Backend Engineer ", ""},
		JobFunctions:    []string{" platform "},
		ExperienceLevel: "",
		Locations:       []string{" Remote "},
		KeywordsInclude: []string{" Go "},
		KeywordsExclude: []string{" PHP "},
		TonePreset:      "",
		ToneAddendum:    " Keep it concise. ",
	})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if record.ExperienceLevel != "mid" {
		t.Fatalf("experience level = %q", record.ExperienceLevel)
	}
	if record.TonePreset != "warm" {
		t.Fatalf("tone preset = %q", record.TonePreset)
	}
	if len(record.JobTitles) != 1 || record.JobTitles[0] != "Backend Engineer" {
		t.Fatalf("job titles = %#v", record.JobTitles)
	}
	if record.ToneAddendum != "Keep it concise." {
		t.Fatalf("tone addendum = %q", record.ToneAddendum)
	}
}

func TestNormalizeRejectsInvalidEnumsAndLongToneAddendum(t *testing.T) {
	if _, err := normalize("user-1", Record{ExperienceLevel: "executive", TonePreset: "warm"}); err == nil {
		t.Fatal("expected invalid experience level error")
	}
	if _, err := normalize("user-1", Record{ExperienceLevel: "senior", TonePreset: "angry"}); err == nil {
		t.Fatal("expected invalid tone preset error")
	}
	if _, err := normalize("user-1", Record{ExperienceLevel: "senior", TonePreset: "warm", ToneAddendum: strings.Repeat("x", 501)}); err == nil {
		t.Fatal("expected long tone addendum error")
	}
}

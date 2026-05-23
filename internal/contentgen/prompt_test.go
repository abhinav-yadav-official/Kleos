package contentgen

import (
	"strings"
	"testing"
)

func TestRenderPromptContainsHardRules(t *testing.T) {
	out, err := RenderPrompt(PromptContext{
		ToneInstruction: TonePresets["warm"],
		ResumeText:      "Jane Doe\nSenior Engineer",
		JobTitle:        "Backend Engineer",
		CompanyName:     "Acme",
		JobDescription:  "Build platform services in Go.",
		RecruiterName:   "Alex",
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	mustContain := []string{
		`"variants"`,
		"HARD RULES",
		"Length: subject 4–9 words",
		"Friendly but professional",
		"RECRUITER_NAME: Alex",
		"COMPANY_NAME: Acme",
		"JOB_TITLE: Backend Engineer",
		"Build platform services in Go.",
		"Jane Doe",
		"Return the JSON now and nothing else.",
	}
	for _, m := range mustContain {
		if !strings.Contains(out, m) {
			t.Errorf("prompt missing %q", m)
		}
	}
	if strings.Contains(out, "USER NOTE") {
		t.Error("USER NOTE should not appear when addendum empty")
	}
}

func TestRenderPromptWithAddendum(t *testing.T) {
	out, _ := RenderPrompt(PromptContext{
		UserAddendum:   "Reference my OSS work when relevant.",
		ResumeText:     "Jane Doe",
		JobTitle:       "X",
		CompanyName:    "Y",
		JobDescription: "Z",
	})
	if !strings.Contains(out, "USER NOTE") {
		t.Error("expected USER NOTE in prompt")
	}
	if !strings.Contains(out, "Reference my OSS work") {
		t.Error("addendum body missing")
	}
}

func TestToneInstructionForFallback(t *testing.T) {
	if ToneInstructionFor("nonsense") != TonePresets["warm"] {
		t.Error("unknown tone should default to warm")
	}
	if ToneInstructionFor("technical") != TonePresets["technical"] {
		t.Error("technical tone wrong")
	}
}

package contentgen

import "testing"

func TestParseGeneratorOutputClean(t *testing.T) {
	raw := []byte(`{"variants":[
		{"subject":"hi","body":"b1"},
		{"subject":"hi2","body":"b2"},
		{"subject":"hi3","body":"b3"}
	]}`)
	r, err := ParseGeneratorOutput(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(r.Variants) != 3 {
		t.Fatalf("got %d variants", len(r.Variants))
	}
}

func TestParseGeneratorOutputWithPreamble(t *testing.T) {
	// Codex --json mode emits JSONL events ending with the final assistant
	// message; we should still extract the variants payload.
	raw := []byte(`{"event":"started","ts":"..."}
{"event":"message","content":{"variants":[
  {"subject":"a b c d","body":"x"},
  {"subject":"a b c d","body":"y"},
  {"subject":"a b c d","body":"z"}
]}}
`)
	r, err := ParseGeneratorOutput(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(r.Variants) != 3 {
		t.Fatalf("got %d variants", len(r.Variants))
	}
}

func TestParseGeneratorOutputBadJSONReturnsError(t *testing.T) {
	raw := []byte(`not even close to json`)
	if _, err := ParseGeneratorOutput(raw); err == nil {
		t.Fatal("expected error")
	}
}

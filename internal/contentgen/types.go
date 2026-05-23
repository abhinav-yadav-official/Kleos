package contentgen

type Variant struct {
	Subject   string  `json:"subject"`
	Body      string  `json:"body"`
	SpamScore float64 `json:"spam_score,omitempty"`
}

type Result struct {
	Variants []Variant `json:"variants"`
}

// PromptContext holds the variables interpolated into the §10 template.
type PromptContext struct {
	ToneInstruction string
	UserAddendum    string
	ResumeText      string
	JobTitle        string
	CompanyName     string
	JobDescription  string
	RecruiterName   string
}

// MaxSpamScore is the threshold above which a generation is treated as
// content_quality failure and not sent.
const MaxSpamScore = 0.30

// TonePresets map preference tone_preset to a one-line instruction inserted
// into the prompt's TONE field. Plan §10.
var TonePresets = map[string]string{
	"formal":    "Professional, courteous, no contractions. Sound like a written letter.",
	"casual":    "Warm and natural. Contractions allowed. Sound like a thoughtful peer reaching out.",
	"technical": "Direct and specific. Lead with technical substance. Minimal pleasantries.",
	"warm":      "Friendly but professional. Brief warmth, then substance. Default.",
}

func ToneInstructionFor(preset string) string {
	if v, ok := TonePresets[preset]; ok {
		return v
	}
	return TonePresets["warm"]
}

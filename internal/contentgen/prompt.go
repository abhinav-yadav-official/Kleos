package contentgen

import (
	"bytes"
	"text/template"
)

// PromptTemplate is the verbatim §10 prompt template. Variables filled by Go.
const PromptTemplate = `You write outreach emails from a job candidate to a hiring contact at a company.
Output STRICT JSON only — no prose before or after. Schema:
{ "variants": [ { "subject": str, "body": str }, { "subject": str, "body": str }, { "subject": str, "body": str } ] }

HARD RULES (every variant must follow all):
1. Length: subject 4–9 words. Body 80–140 words. No exceptions.
2. Plain text only. No markdown. No emojis. No images. No HTML tags. No bullet lists. No tables.
3. Exactly zero links. Exactly zero phone numbers. Exactly zero attachments referenced.
4. No ALL CAPS words longer than 3 characters. No exclamation marks anywhere.
5. No words from this list anywhere (case-insensitive): free, guarantee, urgent, act now, limited time, click here, winner, congratulations, risk-free, no obligation, cash, bonus, opportunity of a lifetime, once in a lifetime, dear friend, dear sir or madam, to whom it may concern, hi dear, amazing, incredible, unbeatable, exclusive deal, special promotion, 100%, $$$, !!!.
6. No tracking language ("did you open this", "as you can see I").
7. Salutation: if RecruiterName is provided and non-empty, "Hi <FirstName>,". Otherwise "Hi <CompanyName> team,". Never "Dear Sir/Madam".
8. First sentence must reference one specific detail from the job description (technology, product area, or stated team challenge). Not generic praise of the company.
9. Second to fourth sentence: 2–3 concrete facts from the candidate's resume that map to the job. Use specific numbers when present (years, scale, metrics).
10. Closing sentence: a single specific ask — a short call or a reply at their convenience. Never two asks.
11. Sign-off: "Best, <CandidateFirstName>" on its own line. The candidate's first name is the first whitespace-separated token of the first non-empty line of the resume that looks like a name (capitalized words, no special characters).
12. Do not invent employment, degrees, or numbers not present in the resume.
13. Do not mention salary, visa, sponsorship, or relocation unless those words appear in the job description.
14. Each of the 3 variants must differ in: (a) the resume detail led with, (b) the closing ask phrasing, (c) the subject line. Avoid near-duplicates.

TONE: {{.ToneInstruction}}
{{ if .UserAddendum }}USER NOTE (apply only if it does not violate HARD RULES): {{.UserAddendum}}{{ end }}

CONTEXT:
RECRUITER_NAME: {{.RecruiterName}}
COMPANY_NAME: {{.CompanyName}}
JOB_TITLE: {{.JobTitle}}
JOB_DESCRIPTION:
"""
{{.JobDescription}}
"""

RESUME (plain text extracted from PDF):
"""
{{.ResumeText}}
"""

Return the JSON now and nothing else.
`

var promptTpl = template.Must(template.New("prompt").Parse(PromptTemplate))

// RenderPrompt fills the §10 template with the given context.
func RenderPrompt(ctx PromptContext) (string, error) {
	if ctx.ToneInstruction == "" {
		ctx.ToneInstruction = TonePresets["warm"]
	}
	var buf bytes.Buffer
	if err := promptTpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

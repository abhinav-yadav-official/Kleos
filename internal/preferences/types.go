package preferences

import "time"

type Record struct {
	UserID          string    `json:"user_id,omitempty"`
	JobTitles       []string  `json:"job_titles"`
	JobFunctions    []string  `json:"job_functions"`
	ExperienceLevel string    `json:"experience_level"`
	Locations       []string  `json:"locations"`
	KeywordsInclude []string  `json:"keywords_include"`
	KeywordsExclude []string  `json:"keywords_exclude"`
	RemoteOnly      bool      `json:"remote_only"`
	TonePreset      string    `json:"tone_preset"`
	ToneAddendum    string    `json:"tone_addendum"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func Default(userID string) Record {
	return Record{
		UserID:          userID,
		JobTitles:       []string{},
		JobFunctions:    []string{},
		ExperienceLevel: "mid",
		Locations:       []string{},
		KeywordsInclude: []string{},
		KeywordsExclude: []string{},
		TonePreset:      "warm",
	}
}

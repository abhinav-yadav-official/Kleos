package campaigns

import "time"

type Campaign struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	ResumeID  string    `json:"resume_id"`
	SMTPID    string    `json:"smtp_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WithCounts struct {
	Campaign
	MatchesByState map[string]int `json:"matches_by_state"`
}

const (
	StatusActive   = "active"
	StatusPaused   = "paused"
	StatusArchived = "archived"
)

type MatchRow struct {
	ID          string    `json:"id"`
	CampaignID  string    `json:"campaign_id"`
	JobID       string    `json:"job_id"`
	MatchScore  float64   `json:"match_score"`
	State       string    `json:"state"`
	MatchedAt   time.Time `json:"matched_at"`
	JobTitle    string    `json:"job_title"`
	JobURL      string    `json:"job_url"`
	JobLocation string    `json:"job_location"`
	JobRemote   bool      `json:"job_remote"`
	JobSource   string    `json:"job_source"`
	CompanyName string    `json:"company_name"`
}

type DraftRow struct {
	ID             string    `json:"id"`
	MatchID        string    `json:"match_id"`
	Chosen         bool      `json:"chosen"`
	SpamScore      float64   `json:"spam_score"`
	Subject        string    `json:"subject"`
	BodyText       string    `json:"body_text"`
	CreatedAt      time.Time `json:"created_at"`
	JobTitle       string    `json:"job_title"`
	CompanyName    string    `json:"company_name"`
	RecruiterEmail string    `json:"recruiter_email"`
}

type SentRow struct {
	ID             string    `json:"id"`
	MatchID        string    `json:"match_id"`
	RecruiterEmail string    `json:"recruiter_email"`
	MessageID      string    `json:"message_id"`
	Status         string    `json:"status"`
	SMTPResponse   string    `json:"smtp_response"`
	SentAt         time.Time `json:"sent_at"`
	JobTitle       string    `json:"job_title"`
	CompanyName    string    `json:"company_name"`
}

func ValidStatus(s string) bool {
	switch s {
	case StatusActive, StatusPaused, StatusArchived:
		return true
	}
	return false
}

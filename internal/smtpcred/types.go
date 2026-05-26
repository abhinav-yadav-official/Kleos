package smtpcred

import "time"

type Credential struct {
	ID         string     `json:"id"`
	UserID     string     `json:"-"`
	Label      string     `json:"label"`
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Username   string     `json:"username"`
	FromEmail  string     `json:"from_email"`
	FromName   string     `json:"from_name,omitempty"`
	UseTLS     bool       `json:"use_tls"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
	LastError  string     `json:"last_error,omitempty"`
	IsPrimary  bool       `json:"is_primary"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CreateInput struct {
	Label     string `json:"label"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
	UseTLS    bool   `json:"use_tls"`
}

// UpdateInput is a partial update for an existing credential. Empty Password
// preserves the stored cipher; any other empty field is ignored. Pointer bools
// allow toggling use_tls without losing the previous value when omitted.
type UpdateInput struct {
	Label     string `json:"label"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromEmail string `json:"from_email"`
	FromName  string `json:"from_name"`
	UseTLS    *bool  `json:"use_tls"`
}

type VerifyResult struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

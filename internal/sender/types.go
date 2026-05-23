package sender

import "time"

const (
	// Warmup defaults from plan §11. Caller may override via config.
	DefaultWarmupDay1Limit = 5
	DefaultWarmupCap       = 40
	DefaultWarmupGrowth    = 1.4
	DefaultWarmupDays      = 21

	// Per-send jitter window (seconds).
	DefaultJitterMinSec = 30
	DefaultJitterMaxSec = 180
)

type ErrorClass int

const (
	// ClassTransient is a 4xx-style SMTP error (graylisted, throttled, etc.).
	// The caller may retry.
	ClassTransient ErrorClass = iota
	// ClassAuthFailure is a permanent authentication or sender-side rejection
	// (e.g. 535 invalid creds, 550 sender domain). Pause the SMTP credential.
	ClassAuthFailure
	// ClassRecipientReject is a permanent recipient-side rejection (e.g. 550
	// mailbox unavailable, 5.1.1). Add recipient to email_denylist.
	ClassRecipientReject
	// ClassUnknown for non-SMTP errors (DNS, TLS, connection reset).
	ClassUnknown
)

func (c ErrorClass) String() string {
	switch c {
	case ClassTransient:
		return "transient"
	case ClassAuthFailure:
		return "auth_failure"
	case ClassRecipientReject:
		return "recipient_reject"
	default:
		return "unknown"
	}
}

// SendResult is what Send returns on success.
type SendResult struct {
	MessageID    string
	SMTPResponse string
	SentAt       time.Time
}

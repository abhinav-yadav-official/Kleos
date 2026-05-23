package sender

import (
	"errors"
	"net/textproto"
	"regexp"
	"strings"
)

var (
	mailboxFullRE = regexp.MustCompile(`(?i)mailbox\s+full|quota`)
	authBadRE     = regexp.MustCompile(`(?i)authentication|invalid\s+credentials|relay\s+access\s+denied|sender\s+address\s+rejected|sender\s+verify`)
	recipientRE   = regexp.MustCompile(`(?i)no\s+such\s+user|user\s+unknown|mailbox\s+unavailable|recipient\s+rejected|address\s+rejected|550 5\.1\.[0-9]|5\.1\.1|invalid recipient|relay denied`)
)

// ClassifyError maps a Go SMTP error to a Phase 5 error class. The classifier
// looks at *textproto.Error codes first (most precise) and falls back to text
// pattern matching on the wrapped message.
func ClassifyError(err error) ErrorClass {
	if err == nil {
		return ClassTransient
	}
	var tp *textproto.Error
	if errors.As(err, &tp) {
		switch {
		case tp.Code >= 400 && tp.Code < 500:
			return ClassTransient
		case tp.Code >= 500 && tp.Code < 600:
			// Use msg patterns to disambiguate auth vs recipient.
			if authBadRE.MatchString(tp.Msg) {
				return ClassAuthFailure
			}
			if recipientRE.MatchString(tp.Msg) {
				return ClassRecipientReject
			}
			// Conservative default: treat as auth so we don't denylist
			// recipients on ambiguous responses.
			return ClassAuthFailure
		}
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "550"):
		if authBadRE.MatchString(msg) {
			return ClassAuthFailure
		}
		if recipientRE.MatchString(msg) || mailboxFullRE.MatchString(msg) {
			return ClassRecipientReject
		}
		return ClassAuthFailure
	case strings.Contains(msg, "535"), strings.Contains(msg, "534"), strings.Contains(msg, "530"):
		return ClassAuthFailure
	case strings.Contains(msg, "421"), strings.Contains(msg, "450"), strings.Contains(msg, "451"), strings.Contains(msg, "452"):
		return ClassTransient
	}
	return ClassUnknown
}

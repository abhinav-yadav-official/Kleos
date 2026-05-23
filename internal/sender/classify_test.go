package sender

import (
	"errors"
	"fmt"
	"net/textproto"
	"testing"
)

func TestClassifyTextproto4xx(t *testing.T) {
	err := &textproto.Error{Code: 421, Msg: "service shutting down"}
	if got := ClassifyError(err); got != ClassTransient {
		t.Errorf("421 = %s, want transient", got)
	}
}

func TestClassifyTextproto5xxAuth(t *testing.T) {
	err := &textproto.Error{Code: 535, Msg: "authentication failed"}
	if got := ClassifyError(err); got != ClassAuthFailure {
		t.Errorf("535 = %s, want auth_failure", got)
	}
}

func TestClassifyTextproto5xxRecipient(t *testing.T) {
	err := &textproto.Error{Code: 550, Msg: "5.1.1 no such user"}
	if got := ClassifyError(err); got != ClassRecipientReject {
		t.Errorf("550 no-such-user = %s, want recipient_reject", got)
	}
}

func TestClassifyWrappedError(t *testing.T) {
	inner := &textproto.Error{Code: 535, Msg: "Invalid credentials"}
	wrapped := fmt.Errorf("send: %w", inner)
	if got := ClassifyError(wrapped); got != ClassAuthFailure {
		t.Errorf("wrapped 535 = %s", got)
	}
}

func TestClassifyPlainString(t *testing.T) {
	err := errors.New("550 mailbox unavailable")
	if got := ClassifyError(err); got != ClassRecipientReject {
		t.Errorf("plain 550 = %s, want recipient_reject", got)
	}
}

func TestClassifySenderPolicyReject(t *testing.T) {
	err := errors.New("close DATA: 550 5.1.1 From header must be equal to sender@example.com")
	if got := ClassifyError(err); got != ClassAuthFailure {
		t.Errorf("sender policy 550 = %s, want auth_failure", got)
	}
}

package sender

import (
	"strings"
	"testing"
	"time"
)

func TestBuildMessageHeaders(t *testing.T) {
	fixedDate := time.Date(2026, 5, 23, 14, 0, 0, 0, time.UTC)
	bytes, id := BuildMessage(Message{
		FromEmail: "me@example.com",
		FromName:  "Alice",
		ToEmail:   "you@otherco.com",
		Subject:   "Quick note about your platform team",
		BodyText:  "Hi You,\n\nLine one.\nLine two.\n\nBest, Alice",
		Date:      fixedDate,
	})
	msg := string(bytes)
	if !strings.Contains(msg, "From: Alice <me@example.com>") {
		t.Errorf("from header wrong:\n%s", msg)
	}
	if !strings.Contains(msg, "To: you@otherco.com") {
		t.Error("to header missing")
	}
	if !strings.Contains(msg, "Subject: Quick note about your platform team") {
		t.Error("subject missing")
	}
	if !strings.Contains(msg, "Message-ID: <"+id+">") {
		t.Errorf("message-id %q not in headers", id)
	}
	if !strings.Contains(msg, "Content-Type: text/plain; charset=UTF-8") {
		t.Error("content-type wrong")
	}
	if !strings.Contains(msg, "Date: Sat, 23 May 2026 14:00:00 +0000") {
		t.Errorf("date header wrong:\n%s", msg)
	}
	if !strings.Contains(msg, "Hi You,\r\n\r\nLine one.\r\nLine two.\r\n\r\nBest, Alice\r\n") {
		t.Errorf("body not CRLF-normalized:\n%q", msg)
	}
}

func TestBuildMessageGeneratesMessageID(t *testing.T) {
	_, id := BuildMessage(Message{FromEmail: "a@b.com", ToEmail: "c@d.com", BodyText: "x"})
	if !strings.HasSuffix(id, "@b.com") {
		t.Errorf("message-id host = %q, want @b.com", id)
	}
	if len(id) < 20 {
		t.Errorf("message-id too short: %q", id)
	}
}

func TestEncodeSubjectNonASCII(t *testing.T) {
	got := encodeSubject("Café meeting")
	if !strings.HasPrefix(got, "=?UTF-8?Q?") {
		t.Errorf("non-ascii subject not q-encoded: %q", got)
	}
}

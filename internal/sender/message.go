package sender

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Message is the minimum data needed to build an RFC 5322 outreach.
type Message struct {
	FromEmail   string
	FromName    string
	ToEmail     string
	Subject     string
	BodyText    string
	Date        time.Time
	MessageID   string // optional; generated if empty
}

// BuildMessage returns the assembled RFC 5322 message bytes plus the Message-ID
// that was used (caller persists it).
func BuildMessage(m Message) ([]byte, string) {
	if m.Date.IsZero() {
		m.Date = time.Now().UTC()
	}
	msgID := m.MessageID
	if msgID == "" {
		msgID = newMessageID(m.FromEmail)
	}
	var b strings.Builder
	fromHeader := m.FromEmail
	if m.FromName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", quoteIfNeeded(m.FromName), m.FromEmail)
	}
	fmt.Fprintf(&b, "From: %s\r\n", fromHeader)
	fmt.Fprintf(&b, "To: %s\r\n", m.ToEmail)
	fmt.Fprintf(&b, "Subject: %s\r\n", encodeSubject(m.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", m.Date.Format(time.RFC1123Z))
	fmt.Fprintf(&b, "Message-ID: <%s>\r\n", msgID)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	b.WriteString("\r\n")
	body := strings.ReplaceAll(m.BodyText, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	b.WriteString(body)
	if !strings.HasSuffix(body, "\r\n") {
		b.WriteString("\r\n")
	}
	return []byte(b.String()), msgID
}

func newMessageID(fromEmail string) string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	domain := "kleos.local"
	if i := strings.IndexByte(fromEmail, '@'); i >= 0 && i < len(fromEmail)-1 {
		domain = fromEmail[i+1:]
	}
	return hex.EncodeToString(buf[:]) + "@" + domain
}

func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, "\",;:<>") {
		s = strings.ReplaceAll(s, `"`, `\"`)
		return `"` + s + `"`
	}
	return s
}

// encodeSubject prefixes non-ASCII subjects with RFC 2047 Q-encoding (UTF-8).
// Subjects in this codebase are ASCII so this is mostly a safety net.
func encodeSubject(s string) string {
	for _, r := range s {
		if r > 127 {
			return "=?UTF-8?Q?" + qEncode(s) + "?="
		}
	}
	return s
}

func qEncode(s string) string {
	var b strings.Builder
	for _, r := range []byte(s) {
		switch {
		case r == ' ':
			b.WriteByte('_')
		case r >= 33 && r <= 126 && r != '=' && r != '?' && r != '_':
			b.WriteByte(r)
		default:
			fmt.Fprintf(&b, "=%02X", r)
		}
	}
	return b.String()
}

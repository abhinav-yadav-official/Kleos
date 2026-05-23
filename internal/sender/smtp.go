package sender

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Credentials is the decrypted set of values pulled from smtp_credentials for
// the active campaign.
type Credentials struct {
	Host      string
	Port      int
	Username  string
	Password  string
	FromEmail string
	FromName  string
	UseTLS    bool
}

// Transport sends a built RFC 5322 message via SMTP. The default impl opens a
// fresh connection per send (sufficient given the low warm-up volume and the
// jitter between sends).
type Transport interface {
	Send(ctx context.Context, c Credentials, to string, msg []byte) (SendResult, error)
}

type NetSMTPTransport struct {
	Dialer  *net.Dialer
	Timeout time.Duration
}

func NewNetSMTPTransport() *NetSMTPTransport {
	return &NetSMTPTransport{Timeout: 30 * time.Second}
}

func (t *NetSMTPTransport) Send(ctx context.Context, c Credentials, to string, msg []byte) (SendResult, error) {
	dialer := t.Dialer
	if dialer == nil {
		dialer = &net.Dialer{Timeout: t.Timeout}
	}
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)

	var conn net.Conn
	var err error
	tlsCfg := &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}
	if c.Port == 465 || (c.UseTLS && c.Port != 587 && c.Port != 25) {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return SendResult{}, fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		return SendResult{}, fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if c.Port != 465 {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				return SendResult{}, fmt.Errorf("starttls: %w", err)
			}
		}
	}

	auth := chooseAuth(c, client)
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return SendResult{}, fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(c.FromEmail); err != nil {
		return SendResult{}, fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return SendResult{}, fmt.Errorf("RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return SendResult{}, fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return SendResult{}, fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return SendResult{}, fmt.Errorf("close DATA: %w", err)
	}
	resp := "250 ok"
	_ = client.Quit()
	return SendResult{SMTPResponse: resp, SentAt: time.Now().UTC()}, nil
}

func chooseAuth(c Credentials, client *smtp.Client) smtp.Auth {
	if c.Username == "" || c.Password == "" {
		return nil
	}
	if ok, mechs := client.Extension("AUTH"); ok {
		m := strings.ToUpper(mechs)
		switch {
		case strings.Contains(m, "PLAIN"):
			return smtp.PlainAuth("", c.Username, c.Password, c.Host)
		case strings.Contains(m, "LOGIN"):
			return smtp.PlainAuth("", c.Username, c.Password, c.Host)
		}
	}
	return smtp.PlainAuth("", c.Username, c.Password, c.Host)
}

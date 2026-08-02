// Package email provides a minimal SMTP client — the app's second
// outbound third-party integration after Ollama, kept just as small
// (stdlib net/smtp, no vendor SDK) since a plain transactional email
// doesn't need one.
package email

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

type Client struct {
	host, port, username, password, from string
}

// NewClient returns an SMTP client. An empty host is a valid, deliberate
// configuration — Send becomes a no-op (logged, not an error) — so the
// app runs fine with email unconfigured (e.g. dev/CI with no mail relay).
func NewClient(host, port, username, password, from string) *Client {
	return &Client{host: host, port: port, username: username, password: password, from: from}
}

// Send delivers a plain-text email to one or more recipients. ctx is
// accepted for interface consistency with the rest of the app's outbound
// clients (e.g. llm.Client) — net/smtp's SendMail has no context support
// of its own to cancel against.
func (c *Client) Send(ctx context.Context, to []string, subject, body string) error {
	if c.host == "" {
		slog.Warn("email not configured, skipping send", "subject", subject, "to", to)
		return nil
	}

	addr := c.host + ":" + c.port
	msg := buildMessage(c.from, to, subject, body)

	var auth smtp.Auth
	if c.username != "" {
		auth = smtp.PlainAuth("", c.username, c.password, c.host)
	}
	if err := smtp.SendMail(addr, auth, c.from, to, msg); err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	return nil
}

// buildMessage composes the raw RFC 5322 message body smtp.SendMail
// expects — split out from Send so it's testable without a live SMTP
// server.
func buildMessage(from string, to []string, subject, body string) []byte {
	return []byte("From: " + from + "\r\n" +
		"To: " + strings.Join(to, ", ") + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" + body + "\r\n")
}

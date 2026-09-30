// Package smtp implements email.Sender against a real SMTP server —
// Mailhog locally (no auth, no TLS — see deployments/docker-compose.yaml's
// mailhog service), a real SMTP relay in prod.
package smtp

import (
	"context"
	"fmt"
	"net/smtp"
)

type Client struct {
	addr string // host:port
	from string
}

func New(host, port, from string) *Client {
	return &Client{addr: host + ":" + port, from: from}
}

// Send has no real cancellation support — net/smtp's SendMail is a
// blocking, non-context-aware stdlib call, so ctx is accepted for
// interface consistency with every other client in this repo but not
// actually wired to anything; a hung SMTP connection blocks the calling
// dispatch attempt until the OS-level TCP timeout, not ctx's deadline.
// Mailhog accepts unauthenticated SMTP, so auth is nil — a real relay in
// prod would need a non-nil smtp.Auth here.
func (c *Client) Send(_ context.Context, to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s\r\n", c.from, to, subject, body)
	if err := smtp.SendMail(c.addr, nil, c.from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("smtp: send mail: %w", err)
	}
	return nil
}

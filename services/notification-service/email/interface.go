// Package email defines the Sender interface usecase/dispatch.go depends
// on; email/smtp, right below this package in the same tree, implements
// it against a real SMTP server (Mailhog locally, per
// docs/ROADMAP.md's Phase 4.1 task list). Keeping the interface here
// instead of off in some unrelated package is just where it belongs — its
// one real implementation lives one directory down.
package email

import "context"

type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

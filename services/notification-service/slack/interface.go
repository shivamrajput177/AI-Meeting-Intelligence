// Package slack defines the Sender interface usecase/dispatch.go depends
// on; slack/http, right below this package in the same tree, implements
// it against a real Slack Incoming Webhook. Keeping the interface here
// instead of off in some unrelated package is just where it belongs — its
// one real implementation lives one directory down.
package slack

import "context"

type Sender interface {
	// Send posts text to webhookURL — a Slack Incoming Webhook URL, per
	// org once Phase 4.5's integration config exists; a single
	// dev-config-wide URL until then (see main.go's doc comment on that
	// simplification).
	Send(ctx context.Context, webhookURL, text string) error
}

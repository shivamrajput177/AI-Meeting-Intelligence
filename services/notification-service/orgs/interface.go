// Package orgs defines the Client interface usecase depends on;
// orgs/http, right below this package in the same tree, implements it
// against Organization Service's internal REST API. Keeping the
// interface here instead of off in some unrelated package is just where
// it belongs — its one real implementation lives one directory down.
package orgs

import "context"

// IntegrationConfig is this service's own Go-to-Go shape for the fields
// DispatchUseCase actually reads off an org's configured integrations —
// not JSON-tagged.
type IntegrationConfig struct {
	SlackWebhookURL *string
	TicketProvider  string
}

type Client interface {
	// GetIntegrationConfig resolves orgID's own Slack webhook URL and
	// ticket provider selection — DispatchUseCase's per-org override of
	// what would otherwise be this service's single dev-config-wide
	// default (see main.go's own doc comment on that Phase 4.1 gap,
	// closed here).
	GetIntegrationConfig(ctx context.Context, orgID string) (*IntegrationConfig, error)
}

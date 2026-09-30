// Package ticketprovider defines the Provider interface DispatchUseCase
// depends on; ticketprovider/mock, right below this package in the same
// tree, implements it against this service's own mock Jira board. Keeping
// the interface here instead of off in some unrelated package is just
// where it belongs. See docs/architecture/deployment-demo-strategy.md §3
// for the full adapter design this backs: a real AtlassianJiraProvider
// (Phase 4.3's stretch job) implements the same interface against Jira
// Cloud's REST API, and DispatchUseCase's ChannelJira case never needs to
// change to support it — only which Provider main.go wires in.
package ticketprovider

import "context"

// TicketRef is what creating a ticket returns — Provider-agnostic enough
// that DispatchUseCase can record it (notification.jira_links) without
// knowing which concrete Provider produced it.
type TicketRef struct {
	Provider string // "mock_jira" | "atlassian_jira" (only "mock_jira" exists as of Phase 4.2)
	Key      string // e.g. "DEMO-142"
	URL      string
}

type Provider interface {
	CreateTicket(ctx context.Context, orgID, actionItemID, title string) (TicketRef, error)
}

// Package repository defines the interfaces usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements them. Keeping the interfaces here instead of off in some
// unrelated package is just where they belong — their one real
// implementation lives one directory down. usecase still only ever
// depends on these interface types, never on the concrete
// *postgres.OutboxRepository/*postgres.JiraRepository directly, which is
// what lets it be unit-tested against an in-memory fake with zero
// Postgres involved. Two interfaces, not one, because Outbox and Jira are
// genuinely distinct data-access concerns (the outbox table vs. the mock
// board's two tables) — the same split auth-service's own
// repository/interface.go uses for its own three distinct concerns.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

type Repository interface {
	// Enqueue inserts a new pending outbox row within its own
	// transaction — see entity.OutboxRow's doc comment: this is the
	// "outbox write" half of the transactional outbox pattern
	// (docs/architecture/microservices.md §10), the durable record that a
	// notification needs to happen, decoupled from actually dispatching
	// it.
	Enqueue(ctx context.Context, orgID, channel string, payload []byte) error

	// ClaimBatch atomically selects up to limit pending rows eligible for
	// (re)attempt — see repository/postgres's own doc comment on the
	// backoff condition — and flips them to StatusSending in the same
	// transaction, so two poller replicas racing on the same tick never
	// both claim the same row (`FOR UPDATE SKIP LOCKED`).
	ClaimBatch(ctx context.Context, limit int) ([]entity.OutboxRow, error)

	// MarkSent records a successful dispatch.
	MarkSent(ctx context.Context, id string) error

	// MarkAttemptFailed records one failed dispatch attempt — permanent
	// transitions the row to StatusFailed (attempts exhausted); otherwise
	// it goes back to StatusPending for the poller's next eligible tick.
	MarkAttemptFailed(ctx context.Context, id string, attempts int, lastErr string, permanent bool) error
}

// JiraRepository backs the mock Jira board and the provider-agnostic
// jira_links bookkeeping — see
// docs/architecture/deployment-demo-strategy.md §3.
type JiraRepository interface {
	// CreateMockIssue inserts a new notification.mock_jira_issues row for
	// actionItemID, generating a sequential per-org issue key (e.g.
	// "DEMO-1", "DEMO-2", ...) — see repository/postgres's own doc
	// comment on the concurrency trade-off this makes.
	CreateMockIssue(ctx context.Context, orgID, actionItemID, title string) (issueKey string, err error)

	// TransitionMockIssue updates issueKey's status and returns the
	// linked action_item_id in the same round trip — the mock board's own
	// card-drag/PATCH endpoint, which needs that id to write the mapped
	// status back onto the action item (see
	// deployment-demo-strategy.md §3's "genuinely bidirectional" design).
	TransitionMockIssue(ctx context.Context, orgID, issueKey, newStatus string) (actionItemID string, err error)

	// ListMockBoard returns every mock issue for orgID, oldest first, for
	// the board views (GET /demo/board, GET /orgs/{orgId}/mock-jira/board).
	ListMockBoard(ctx context.Context, orgID string) ([]entity.MockJiraIssue, error)

	// UpsertJiraLink records which provider created which ticket for
	// actionItemID — provider-agnostic bookkeeping DispatchUseCase writes
	// after any ticketprovider.Provider.CreateTicket call succeeds, mock
	// or (once Phase 4.3 lands) real.
	UpsertJiraLink(ctx context.Context, orgID, actionItemID, provider, issueKey, url string) error
}

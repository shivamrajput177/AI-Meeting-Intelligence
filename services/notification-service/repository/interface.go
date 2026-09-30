// Package repository defines the interfaces usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements them. Keeping the interfaces here instead of off in some
// unrelated package is just where they belong — their one real
// implementation lives one directory down. usecase still only ever
// depends on these interface types, never on the concrete
// *postgres.OutboxRepository/*postgres.JiraRepository/*postgres.ReminderRepository
// directly, which is what lets it be unit-tested against an in-memory
// fake with zero Postgres involved. Three interfaces, not one, because
// Outbox, Jira, and Reminders are genuinely distinct data-access concerns
// — the same split auth-service's own repository/interface.go uses for
// its own three distinct concerns.
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

// ReminderRepository backs the reminder scheduler
// (docs/architecture/microservices.md §10) — the one deliberate exception
// to this codebase's "services never touch each other's tables, only
// REST/Kafka" rule (see docs/architecture/microservices.md §"Internal
// Communication"): actionitem.reminders lives in Action Item Service's
// own schema, but every service already shares one physical Postgres
// instance/database and connects as the same superuser (the same fact
// that made RLS a no-op, per database-schema.md's "Row-Level Security
// pattern" section), and the architecture doc's own reminder-scheduler
// design has Notification Service query that table directly rather than
// through a synthesized REST endpoint — a real, if unusual, documented
// design decision, not an accidental layering violation.
type ReminderRepository interface {
	// WithLeaderLock runs fn only on the one replica that wins a
	// cluster-wide Postgres advisory lock this tick — see
	// repository/postgres's own doc comment for the pg_try_advisory_lock/
	// pg_advisory_unlock mechanics. Every replica can call this every
	// tick; at most one's fn ever actually runs per tick, and any replica
	// can win the next one (no dedicated singleton pod needed, per
	// microservices.md §10's own "Scaling" note).
	WithLeaderLock(ctx context.Context, fn func(ctx context.Context) error) error

	// ClaimDueReminders selects up to limit reminders whose remind_at has
	// passed and that haven't been sent yet, joined with
	// actionitem.action_items for the org_id/description/owner a Slack
	// message needs — see entity.DueReminder's own doc comment.
	ClaimDueReminders(ctx context.Context, limit int) ([]entity.DueReminder, error)

	// MarkReminderSent records that a reminder was successfully
	// published to action-item.reminder-due.v1 — never re-claimed by
	// ClaimDueReminders again after this.
	MarkReminderSent(ctx context.Context, id string) error
}

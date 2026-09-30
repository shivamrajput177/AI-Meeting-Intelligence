// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on
// *postgres.OutboxRepository directly, which is what lets it be
// unit-tested against an in-memory fake with zero Postgres involved.
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

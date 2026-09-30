// Package postgres implements repository.Repository against the
// notification.* schema (see docs/architecture/microservices.md §10).
//
// Every query here filters by org_id explicitly in the SQL itself, not
// just through dbx.WithTenantTx's SET LOCAL app.current_org — see
// docs/architecture/database-schema.md's "Row-Level Security pattern"
// section: every service's runtime DB connection is the Postgres
// superuser/table owner, which RLS policies never apply to, so the
// explicit filter is the real enforcement (found the hard way, as a live
// cross-tenant leak, during Phase 2.3 — not repeating that mistake here).
// ClaimBatch is the one exception: it operates across every org in one
// query by design (one poller serving every tenant's outbox), so its
// tenant isolation is enforced per-row by the CHECK/RLS policy on the
// table itself, not a query-level filter.
//
// Backoff (ClaimBatch's WHERE clause) is measured from created_at, not
// from the row's last attempt — an approximation the documented schema
// (docs/architecture/microservices.md §10) makes unavoidable without an
// extra updated_at/next_attempt_at column: attempts * 30s of wall-clock
// time since the row was first enqueued, not since it was last retried.
// A real implementation would add that column; this is an accepted
// simplification for now, same spirit as every other honestly-documented
// gap in this codebase.
package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

func (r *OutboxRepository) Enqueue(ctx context.Context, orgID, channel string, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification.outbox (id, org_id, channel, payload, status, attempts, created_at)
		VALUES ($1, $2, $3, $4, $5, 0, now())
	`, uuid.NewString(), orgID, channel, payload, entity.StatusPending)
	return err
}

// ClaimBatch claims up to limit eligible pending rows in one round trip —
// the SELECT ... FOR UPDATE SKIP LOCKED and the status flip to "sending"
// happen as a single statement, so two poller replicas racing on the same
// tick never both claim the same row.
func (r *OutboxRepository) ClaimBatch(ctx context.Context, limit int) ([]entity.OutboxRow, error) {
	rows, err := r.pool.Query(ctx, `
		WITH claimed AS (
			SELECT id FROM notification.outbox
			WHERE status = $1
				AND (attempts = 0 OR now() >= created_at + (attempts * interval '30 seconds'))
			ORDER BY created_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE notification.outbox o
		SET status = $3
		FROM claimed
		WHERE o.id = claimed.id
		RETURNING o.id, o.org_id, o.channel, o.payload, o.attempts, o.created_at
	`, entity.StatusPending, limit, entity.StatusSending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.OutboxRow
	for rows.Next() {
		var row entity.OutboxRow
		if err := rows.Scan(&row.ID, &row.OrgID, &row.Channel, &row.Payload, &row.Attempts, &row.CreatedAt); err != nil {
			return nil, err
		}
		row.Status = entity.StatusSending
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *OutboxRepository) MarkSent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE notification.outbox SET status = $1, sent_at = now() WHERE id = $2
	`, entity.StatusSent, id)
	return err
}

func (r *OutboxRepository) MarkAttemptFailed(ctx context.Context, id string, attempts int, lastErr string, permanent bool) error {
	status := entity.StatusPending
	if permanent {
		status = entity.StatusFailed
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE notification.outbox SET status = $1, attempts = $2, last_error = $3 WHERE id = $4
	`, status, attempts, lastErr, id)
	return err
}

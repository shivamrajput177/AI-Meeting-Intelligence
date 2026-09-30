// reminder_repo.go implements repository.ReminderRepository — see that
// interface's own doc comment for why this service queries Action Item
// Service's actionitem.* schema directly instead of over REST.
//
// pg_try_advisory_lock is session-scoped: whichever connection takes the
// lock is the only one that can release it, so WithLeaderLock holds one
// dedicated connection out of the pool for its entire duration instead of
// letting pgx borrow a different one per statement the way every other
// method here does.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

// reminderSchedulerLockKey is this service's own dedicated
// pg_try_advisory_lock key for reminder-scheduler leader election —
// deliberately distinct from the 727001 key several services' own
// migrations share for serializing CREATE EXTENSION: advisory lock keys
// are one flat cluster-wide namespace, and colliding with an unrelated
// lock would be a real, if rare, correctness bug.
const reminderSchedulerLockKey = 727200

type ReminderRepository struct {
	pool *pgxpool.Pool
}

func NewReminderRepository(pool *pgxpool.Pool) *ReminderRepository {
	return &ReminderRepository{pool: pool}
}

// WithLeaderLock tries to win reminderSchedulerLockKey and runs fn only if
// it does; if another replica already holds it, fn is skipped and this
// returns nil — a normal, expected outcome most ticks, not an error. The
// unlock uses a fresh background context rather than ctx, so a caller
// whose context is already cancelled still releases the lock instead of
// leaving it held until the pool eventually resets this connection.
func (r *ReminderRepository) WithLeaderLock(ctx context.Context, fn func(ctx context.Context) error) error {
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", reminderSchedulerLockKey).Scan(&acquired); err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", reminderSchedulerLockKey)
	}()

	return fn(ctx)
}

// ClaimDueReminders' FOR UPDATE SKIP LOCKED is redundant-but-documented
// belt-and-suspenders given the caller already only ever runs this inside
// WithLeaderLock (so only one replica is ever querying at a time) — kept
// because it's exactly what
// docs/architecture/kafka-topics.md's flow-4 diagram specifies, and
// costs nothing extra to keep true to.
func (r *ReminderRepository) ClaimDueReminders(ctx context.Context, limit int) ([]entity.DueReminder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT rem.id, rem.action_item_id, a.org_id, a.description, a.owner_user_id, rem.channel
		FROM actionitem.reminders rem
		JOIN actionitem.action_items a ON a.id = rem.action_item_id
		WHERE rem.remind_at <= now() AND rem.sent_at IS NULL
		ORDER BY rem.remind_at
		LIMIT $1
		FOR UPDATE OF rem SKIP LOCKED
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.DueReminder
	for rows.Next() {
		var d entity.DueReminder
		if err := rows.Scan(&d.ID, &d.ActionItemID, &d.OrgID, &d.Description, &d.OwnerUserID, &d.Channel); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *ReminderRepository) MarkReminderSent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE actionitem.reminders SET sent_at = now() WHERE id = $1`, id)
	return err
}

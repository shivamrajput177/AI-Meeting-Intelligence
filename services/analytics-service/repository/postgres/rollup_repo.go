// Package postgres implements repository.Repository against the
// analytics.* schema (see docs/architecture/microservices.md §11).
//
// Every query here filters by org_id explicitly in the SQL itself, not
// just through dbx.WithTenantTx's SET LOCAL app.current_org — see
// docs/architecture/database-schema.md's "Row-Level Security pattern"
// section: every service's runtime DB connection is the Postgres
// superuser/table owner, which RLS policies never apply to, so the
// explicit filter is the real enforcement (found the hard way, as a live
// cross-tenant leak, during Phase 2.3 — not repeating that mistake here).
//
// Every write is an idempotent upsert keyed by each table's own PK —
// docs/architecture/microservices.md §11's "Scaling" note calls this out
// specifically: at-least-once Kafka delivery redelivering the same event
// just re-applies the same increment twice unless the caller already
// de-duped, which none of these do. That's a real, accepted gap (a
// redelivered event double-counts a rollup), traded for not needing a
// separate dedup table for what's explicitly a disposable/replayable read
// model.
package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
)

type RollupRepository struct {
	pool *pgxpool.Pool
}

func NewRollupRepository(pool *pgxpool.Pool) *RollupRepository {
	return &RollupRepository{pool: pool}
}

func (r *RollupRepository) UpsertMeetingCompletion(ctx context.Context, orgID string, day time.Time, durationMinutes int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO analytics.meeting_daily_rollup (org_id, day, meeting_count, total_minutes)
		VALUES ($1, $2, 1, $3)
		ON CONFLICT (org_id, day) DO UPDATE SET
			meeting_count = analytics.meeting_daily_rollup.meeting_count + 1,
			total_minutes = analytics.meeting_daily_rollup.total_minutes + EXCLUDED.total_minutes
	`, orgID, day, durationMinutes)
	return err
}

func (r *RollupRepository) IncrementActionItemOpened(ctx context.Context, orgID, ownerUserID string, day time.Time) error {
	return r.incrementActionItem(ctx, orgID, ownerUserID, day, "opened")
}

func (r *RollupRepository) IncrementActionItemClosed(ctx context.Context, orgID, ownerUserID string, day time.Time) error {
	return r.incrementActionItem(ctx, orgID, ownerUserID, day, "closed")
}

// incrementActionItem is shared by IncrementActionItemOpened/Closed —
// column is always one of the two literal names below (never
// caller-supplied), so interpolating it into the query string carries no
// injection risk.
func (r *RollupRepository) incrementActionItem(ctx context.Context, orgID, ownerUserID string, day time.Time, column string) error {
	if column != "opened" && column != "closed" {
		panic("analytics: incrementActionItem: invalid column " + column)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO analytics.action_item_rollup (org_id, owner_user_id, day, opened, closed)
		VALUES ($1, $2, $3, CASE WHEN $4 = 'opened' THEN 1 ELSE 0 END, CASE WHEN $4 = 'closed' THEN 1 ELSE 0 END)
		ON CONFLICT (org_id, owner_user_id, day) DO UPDATE SET
			opened = analytics.action_item_rollup.opened + (CASE WHEN $4 = 'opened' THEN 1 ELSE 0 END),
			closed = analytics.action_item_rollup.closed + (CASE WHEN $4 = 'closed' THEN 1 ELSE 0 END)
	`, orgID, ownerUserID, day, column)
	return err
}

func (r *RollupRepository) IncrementTopicMentions(ctx context.Context, orgID, topic string, week time.Time, count int) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO analytics.topic_frequency (org_id, topic, week, mentions)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (org_id, topic, week) DO UPDATE SET
			mentions = analytics.topic_frequency.mentions + EXCLUDED.mentions
	`, orgID, topic, week, count)
	return err
}

func (r *RollupRepository) GetMeetingTrends(ctx context.Context, orgID string, sinceDays int) ([]entity.MeetingDailyRollup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT day, meeting_count, total_minutes FROM analytics.meeting_daily_rollup
		WHERE org_id = $1 AND day >= (now() - make_interval(days => $2))::date
		ORDER BY day ASC
	`, orgID, sinceDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.MeetingDailyRollup
	for rows.Next() {
		var row entity.MeetingDailyRollup
		row.OrgID = orgID
		if err := rows.Scan(&row.Day, &row.MeetingCount, &row.TotalMinutes); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *RollupRepository) GetProductivity(ctx context.Context, orgID string) ([]entity.ActionItemRollup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT owner_user_id, SUM(opened)::int, SUM(closed)::int FROM analytics.action_item_rollup
		WHERE org_id = $1
		GROUP BY owner_user_id
		ORDER BY SUM(opened) DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.ActionItemRollup
	for rows.Next() {
		var row entity.ActionItemRollup
		row.OrgID = orgID
		if err := rows.Scan(&row.OwnerUserID, &row.Opened, &row.Closed); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r *RollupRepository) GetCompletionRate(ctx context.Context, orgID string) (opened, closed int, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(opened), 0)::int, COALESCE(SUM(closed), 0)::int
		FROM analytics.action_item_rollup WHERE org_id = $1
	`, orgID).Scan(&opened, &closed)
	return opened, closed, err
}

func (r *RollupRepository) GetTopTopics(ctx context.Context, orgID string, sinceWeeks, limit int) ([]entity.TopicFrequency, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT topic, SUM(mentions)::int AS total FROM analytics.topic_frequency
		WHERE org_id = $1 AND week >= (now() - make_interval(weeks => $2))::date
		GROUP BY topic
		ORDER BY total DESC
		LIMIT $3
	`, orgID, sinceWeeks, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.TopicFrequency
	for rows.Next() {
		var row entity.TopicFrequency
		row.OrgID = orgID
		if err := rows.Scan(&row.Topic, &row.Mentions); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

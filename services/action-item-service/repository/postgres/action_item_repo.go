// Package postgres implements repository.Repository against the
// actionitem.* schema (see docs/architecture/microservices.md §8).
//
// Every query here filters by org_id explicitly in the SQL itself, not
// just through dbx.WithTenantTx's SET LOCAL app.current_org — see
// docs/architecture/database-schema.md's "Row-Level Security pattern"
// section: every service's runtime DB connection is the Postgres
// superuser/table owner, which RLS policies never apply to, so the
// explicit filter is the real enforcement (found the hard way, as a live
// cross-tenant leak, during Phase 2.3 — not repeating that mistake here).
package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
)

type ActionItemRepository struct {
	pool *pgxpool.Pool
}

func NewActionItemRepository(pool *pgxpool.Pool) *ActionItemRepository {
	return &ActionItemRepository{pool: pool}
}

const selectCols = `id, meeting_id, org_id, description, type, owner_user_id, owner_raw_name,
	due_date, status, priority, jira_issue_key, extracted_from_chunk_id, confidence, created_at, updated_at`

func scanActionItem(row pgx.Row) (*entity.ActionItem, error) {
	var it entity.ActionItem
	err := row.Scan(
		&it.ID, &it.MeetingID, &it.OrgID, &it.Description, &it.Type, &it.OwnerUserID, &it.OwnerRawName,
		&it.DueDate, &it.Status, &it.Priority, &it.JiraIssueKey, &it.ExtractedFromChunkID, &it.Confidence,
		&it.CreatedAt, &it.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("action item not found")
	}
	return &it, err
}

// reminderLeadHoursUTC is how far past midnight UTC on an action item's
// due date its reminder fires — a fixed, documented default (a real
// product would likely make this configurable per-org, a Phase 4.5-ish
// follow-up, not built speculatively here).
const reminderLeadHoursUTC = 9

func (r *ActionItemRepository) ReplaceActionItems(ctx context.Context, orgID, meetingID string, items []*entity.ActionItem) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		// actionitem.reminders' ON DELETE CASCADE FK means deleting a
		// meeting's action items here already deletes their reminders too
		// — re-extraction (a summary regenerate) shouldn't leave stale
		// reminders from a previous pass any more than it should leave
		// stale action items.
		if _, err := tx.Exec(ctx, `DELETE FROM actionitem.action_items WHERE meeting_id = $1 AND org_id = $2`, meetingID, orgID); err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}

		batch := &pgx.Batch{}
		for _, it := range items {
			batch.Queue(
				`INSERT INTO actionitem.action_items
				 (id, meeting_id, org_id, description, type, owner_user_id, owner_raw_name, due_date,
				  status, priority, extracted_from_chunk_id, confidence, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now(), now())`,
				it.ID, it.MeetingID, orgID, it.Description, it.Type, it.OwnerUserID, it.OwnerRawName, it.DueDate,
				it.Status, it.Priority, it.ExtractedFromChunkID, it.Confidence,
			)
			// A reminder is only ever created here, for an item that has
			// a due date — see docs/architecture/microservices.md §10's
			// reminder scheduler design and this repo's own doc comment
			// on reminderLeadHoursUTC. Nothing else in this codebase
			// creates a reminders row yet (no REST endpoint for it is
			// documented), so extraction time is the one real trigger.
			if it.DueDate != nil {
				batch.Queue(
					`INSERT INTO actionitem.reminders (id, action_item_id, remind_at, channel)
					 VALUES ($1, $2, $3::date::timestamptz + make_interval(hours => $4), 'slack')`,
					uuid.NewString(), it.ID, it.DueDate, reminderLeadHoursUTC,
				)
			}
		}
		br := tx.SendBatch(ctx, batch)
		defer func() { _ = br.Close() }()
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *ActionItemRepository) GetByID(ctx context.Context, orgID, id string) (*entity.ActionItem, error) {
	var item *entity.ActionItem
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var scanErr error
		item, scanErr = scanActionItem(tx.QueryRow(ctx,
			`SELECT `+selectCols+` FROM actionitem.action_items WHERE id = $1 AND org_id = $2`, id, orgID))
		return scanErr
	})
	return item, err
}

func (r *ActionItemRepository) ListByMeeting(ctx context.Context, orgID, meetingID string) ([]*entity.ActionItem, error) {
	var items []*entity.ActionItem
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT `+selectCols+` FROM actionitem.action_items WHERE meeting_id = $1 AND org_id = $2 ORDER BY created_at`,
			meetingID, orgID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			it, err := scanActionItem(rows)
			if err != nil {
				return err
			}
			items = append(items, it)
		}
		return rows.Err()
	})
	return items, err
}

// List is the cross-meeting GET /action-items query — filters are applied
// only when set (an empty ListActionItemsFilter returns every item in the
// org, paginated).
func (r *ActionItemRepository) List(ctx context.Context, orgID string, filter entity.ListActionItemsFilter) ([]*entity.ActionItem, int, error) {
	var items []*entity.ActionItem
	var total int
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		where := `WHERE org_id = $1`
		args := []any{orgID}
		if filter.OwnerUserID != "" {
			args = append(args, filter.OwnerUserID)
			where += ` AND owner_user_id = $` + strconv.Itoa(len(args))
		}
		if filter.Status != "" {
			args = append(args, filter.Status)
			where += ` AND status = $` + strconv.Itoa(len(args))
		}
		if filter.Type != "" {
			args = append(args, filter.Type)
			where += ` AND type = $` + strconv.Itoa(len(args))
		}
		if filter.DueBefore != nil {
			args = append(args, *filter.DueBefore)
			where += ` AND due_date <= $` + strconv.Itoa(len(args))
		}

		if err := tx.QueryRow(ctx, `SELECT count(*) FROM actionitem.action_items `+where, args...).Scan(&total); err != nil {
			return err
		}

		offset := (filter.Page - 1) * filter.PageSize
		args = append(args, filter.PageSize, offset)
		rows, err := tx.Query(ctx,
			`SELECT `+selectCols+` FROM actionitem.action_items `+where+
				` ORDER BY created_at DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)),
			args...,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			it, err := scanActionItem(rows)
			if err != nil {
				return err
			}
			items = append(items, it)
		}
		return rows.Err()
	})
	return items, total, err
}

func (r *ActionItemRepository) Update(ctx context.Context, orgID, id string, input entity.UpdateActionItemInput) (*entity.ActionItem, error) {
	var item *entity.ActionItem
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE actionitem.action_items SET
			   status = COALESCE($1, status),
			   owner_user_id = COALESCE($2, owner_user_id),
			   due_date = COALESCE($3, due_date),
			   jira_issue_key = COALESCE($4, jira_issue_key),
			   updated_at = now()
			 WHERE id = $5 AND org_id = $6`,
			input.Status, input.OwnerUserID, input.DueDate, input.JiraIssueKey, id, orgID,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound("action item not found")
		}

		var scanErr error
		item, scanErr = scanActionItem(tx.QueryRow(ctx,
			`SELECT `+selectCols+` FROM actionitem.action_items WHERE id = $1 AND org_id = $2`, id, orgID))
		return scanErr
	})
	return item, err
}

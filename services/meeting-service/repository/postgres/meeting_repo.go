// Package postgres implements repository.Repository against the
// meeting.* schema (see docs/architecture/database-schema.md).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
)

type MeetingRepository struct {
	pool *pgxpool.Pool
}

func NewMeetingRepository(pool *pgxpool.Pool) *MeetingRepository {
	return &MeetingRepository{pool: pool}
}

func (r *MeetingRepository) Create(ctx context.Context, m *entity.Meeting) error {
	return dbx.WithTenantTx(ctx, r.pool, m.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO meeting.meetings
			 (id, org_id, title, created_by, status, source_type, recording_object_key, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			m.ID, m.OrgID, m.Title, m.CreatedBy, m.Status, m.SourceType, m.RecordingObjectKey,
			m.CreatedAt, m.UpdatedAt,
		)
		return err
	})
}

func scanMeeting(row pgx.Row) (*entity.Meeting, error) {
	var m entity.Meeting
	err := row.Scan(&m.ID, &m.OrgID, &m.Title, &m.CreatedBy, &m.Status, &m.SourceType,
		&m.RecordingObjectKey, &m.DurationSeconds, &m.StartedAt, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("meeting not found")
	}
	return &m, err
}

const selectMeetingCols = `id, org_id, title, created_by, status, source_type, recording_object_key,
	duration_seconds, started_at, created_at, updated_at`

// Every query below filters by org_id explicitly, not just through
// WithTenantTx's SET LOCAL app.current_org — this service's runtime DB
// connection is the Postgres superuser/table owner (see database_url in
// deployments/configs/meeting-service.*), and RLS policies don't apply
// to the table owner by design (see
// docs/architecture/database-schema.md's "Row-Level Security pattern"
// note). Without the explicit filter these are real cross-tenant IDOR
// and full-table-scan leaks, not just theoretical: confirmed live during
// Phase 2.3 by fetching another org's meeting through this exact code
// path before this fix.
func (r *MeetingRepository) GetByID(ctx context.Context, orgID, id string) (*entity.Meeting, error) {
	var meeting *entity.Meeting
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var scanErr error
		meeting, scanErr = scanMeeting(tx.QueryRow(ctx,
			`SELECT `+selectMeetingCols+` FROM meeting.meetings WHERE id = $1 AND org_id = $2`, id, orgID))
		return scanErr
	})
	return meeting, err
}

func (r *MeetingRepository) List(ctx context.Context, orgID string, filter entity.ListFilter) ([]*entity.Meeting, int, error) {
	var items []*entity.Meeting
	var total int
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM meeting.meetings WHERE org_id = $1`, orgID).Scan(&total); err != nil {
			return err
		}

		offset := (filter.Page - 1) * filter.PageSize
		rows, err := tx.Query(ctx,
			`SELECT `+selectMeetingCols+` FROM meeting.meetings WHERE org_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
			orgID, filter.PageSize, offset,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			m, err := scanMeeting(rows)
			if err != nil {
				return err
			}
			items = append(items, m)
		}
		return rows.Err()
	})
	return items, total, err
}

func (r *MeetingRepository) UpdateStatus(ctx context.Context, orgID, id, status string) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE meeting.meetings SET status = $1, updated_at = now() WHERE id = $2 AND org_id = $3`, status, id, orgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound("meeting not found")
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO meeting.status_history (meeting_id, status, changed_at) VALUES ($1, $2, now())`,
			id, status)
		return err
	})
}

func (r *MeetingRepository) Touch(ctx context.Context, orgID, id string) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE meeting.meetings SET updated_at = now() WHERE id = $1 AND org_id = $2`, id, orgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound("meeting not found")
		}
		return nil
	})
}

// ListParticipants joins through meeting.meetings for org scoping — see
// repository.Repository.ListParticipants's doc comment for why (the
// participants table itself carries no org_id).
func (r *MeetingRepository) ListParticipants(ctx context.Context, orgID, meetingID string) ([]*entity.Participant, error) {
	var items []*entity.Participant
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT p.user_id, p.email, p.display_name
			 FROM meeting.participants p
			 JOIN meeting.meetings m ON m.id = p.meeting_id
			 WHERE p.meeting_id = $1 AND m.org_id = $2`,
			meetingID, orgID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p entity.Participant
			var displayName *string
			if err := rows.Scan(&p.UserID, &p.Email, &displayName); err != nil {
				return err
			}
			if displayName != nil {
				p.DisplayName = *displayName
			}
			items = append(items, &p)
		}
		return rows.Err()
	})
	return items, err
}

func (r *MeetingRepository) Delete(ctx context.Context, orgID, id string) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM meeting.meetings WHERE id = $1 AND org_id = $2`, id, orgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return apperr.NotFound("meeting not found")
		}
		return nil
	})
}

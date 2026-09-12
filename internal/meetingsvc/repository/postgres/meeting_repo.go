// Package postgres implements meetingsvc/domain.Repository against the
// meeting.* schema (see docs/architecture/database-schema.md).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/dbx"
)

type MeetingRepository struct {
	pool *pgxpool.Pool
}

func NewMeetingRepository(pool *pgxpool.Pool) *MeetingRepository {
	return &MeetingRepository{pool: pool}
}

func (r *MeetingRepository) Create(ctx context.Context, m *domain.Meeting) error {
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

func scanMeeting(row pgx.Row) (*domain.Meeting, error) {
	var m domain.Meeting
	err := row.Scan(&m.ID, &m.OrgID, &m.Title, &m.CreatedBy, &m.Status, &m.SourceType,
		&m.RecordingObjectKey, &m.DurationSeconds, &m.StartedAt, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrMeetingNotFound
	}
	return &m, err
}

const selectMeetingCols = `id, org_id, title, created_by, status, source_type, recording_object_key,
	duration_seconds, started_at, created_at, updated_at`

func (r *MeetingRepository) GetByID(ctx context.Context, orgID, id string) (*domain.Meeting, error) {
	var meeting *domain.Meeting
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		var scanErr error
		meeting, scanErr = scanMeeting(tx.QueryRow(ctx,
			`SELECT `+selectMeetingCols+` FROM meeting.meetings WHERE id = $1`, id))
		return scanErr
	})
	return meeting, err
}

func (r *MeetingRepository) List(ctx context.Context, orgID string, filter domain.ListFilter) ([]*domain.Meeting, int, error) {
	var items []*domain.Meeting
	var total int
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM meeting.meetings`).Scan(&total); err != nil {
			return err
		}

		offset := (filter.Page - 1) * filter.PageSize
		rows, err := tx.Query(ctx,
			`SELECT `+selectMeetingCols+` FROM meeting.meetings ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
			filter.PageSize, offset,
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
			`UPDATE meeting.meetings SET status = $1, updated_at = now() WHERE id = $2`, status, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrMeetingNotFound
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO meeting.status_history (meeting_id, status, changed_at) VALUES ($1, $2, now())`,
			id, status)
		return err
	})
}

func (r *MeetingRepository) Touch(ctx context.Context, orgID, id string) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE meeting.meetings SET updated_at = now() WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrMeetingNotFound
		}
		return nil
	})
}

func (r *MeetingRepository) Delete(ctx context.Context, orgID, id string) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM meeting.meetings WHERE id = $1`, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrMeetingNotFound
		}
		return nil
	})
}

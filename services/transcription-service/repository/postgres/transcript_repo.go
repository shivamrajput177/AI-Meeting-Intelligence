// Package postgres implements repository.Repository against the
// transcription.* schema (see docs/architecture/microservices.md §6).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
)

type TranscriptRepository struct {
	pool *pgxpool.Pool
}

func NewTranscriptRepository(pool *pgxpool.Pool) *TranscriptRepository {
	return &TranscriptRepository{pool: pool}
}

func (r *TranscriptRepository) CreateTranscript(ctx context.Context, t *entity.Transcript) error {
	return dbx.WithTenantTx(ctx, r.pool, t.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO transcription.transcripts
			 (id, meeting_id, org_id, language, engine, model_name, status, raw_text, word_count, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			t.ID, t.MeetingID, t.OrgID, t.Language, t.Engine, t.ModelName, t.Status, t.RawText, t.WordCount, t.CreatedAt,
		)
		return err
	})
}

// CreateSegments batches every segment for one transcript into a single
// round trip — a transcript can easily have hundreds of segments, and
// this is called once per transcription, not per segment.
func (r *TranscriptRepository) CreateSegments(ctx context.Context, orgID string, segments []*entity.Segment) error {
	if len(segments) == 0 {
		return nil
	}
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, s := range segments {
			batch.Queue(
				`INSERT INTO transcription.segments (transcript_id, speaker_label, start_ms, end_ms, text, confidence)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				s.TranscriptID, s.SpeakerLabel, s.StartMS, s.EndMS, s.Text, s.Confidence,
			)
		}
		br := tx.SendBatch(ctx, batch)
		defer func() { _ = br.Close() }()
		for range segments {
			if _, err := br.Exec(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *TranscriptRepository) GetByMeetingID(ctx context.Context, orgID, meetingID string) (*entity.Transcript, []*entity.Segment, error) {
	var t entity.Transcript
	var segments []*entity.Segment
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		// org_id is filtered explicitly here, not left to RLS alone:
		// every service's runtime DB connection in this project is the
		// Postgres superuser/table owner (see database_url in
		// deployments/configs/*), and Postgres RLS policies don't apply
		// to the table owner by design (see
		// docs/architecture/database-schema.md's own "Row-Level Security
		// pattern" note) — RLS is silently a no-op for every table in
		// this system today, not just this one. This WHERE clause is the
		// real enforcement for this query; WithTenantTx's SET LOCAL
		// above is closer to a no-op given that gap.
		if err := tx.QueryRow(ctx,
			`SELECT id, meeting_id, org_id, COALESCE(language, ''), engine, model_name, status, raw_text, word_count, created_at
			 FROM transcription.transcripts WHERE meeting_id = $1 AND org_id = $2`,
			meetingID, orgID,
		).Scan(&t.ID, &t.MeetingID, &t.OrgID, &t.Language, &t.Engine, &t.ModelName, &t.Status, &t.RawText, &t.WordCount, &t.CreatedAt); err != nil {
			return err
		}

		rows, err := tx.Query(ctx,
			`SELECT id, transcript_id, COALESCE(speaker_label, ''), start_ms, end_ms, text, COALESCE(confidence, 0)
			 FROM transcription.segments WHERE transcript_id = $1 ORDER BY start_ms ASC`,
			t.ID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var s entity.Segment
			if err := rows.Scan(&s.ID, &s.TranscriptID, &s.SpeakerLabel, &s.StartMS, &s.EndMS, &s.Text, &s.Confidence); err != nil {
				return err
			}
			segments = append(segments, &s)
		}
		return rows.Err()
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, apperr.NotFound("transcript not found")
	}
	if err != nil {
		return nil, nil, err
	}
	return &t, segments, nil
}

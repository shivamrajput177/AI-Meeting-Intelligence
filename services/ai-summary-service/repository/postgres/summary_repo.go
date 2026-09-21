// Package postgres implements repository.Repository against the ai.*
// schema (see docs/architecture/microservices.md §7).
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
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
)

type SummaryRepository struct {
	pool *pgxpool.Pool
}

func NewSummaryRepository(pool *pgxpool.Pool) *SummaryRepository {
	return &SummaryRepository{pool: pool}
}

func (r *SummaryRepository) UpsertSummary(ctx context.Context, orgID string, s *entity.Summary) error {
	keyDecisions, err := json.Marshal(s.KeyDecisions)
	if err != nil {
		return err
	}
	risks, err := json.Marshal(s.Risks)
	if err != nil {
		return err
	}
	blockers, err := json.Marshal(s.Blockers)
	if err != nil {
		return err
	}

	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO ai.summaries (id, meeting_id, org_id, summary_text, key_decisions, risks, blockers, model_used, prompt_version, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			 ON CONFLICT (meeting_id) DO UPDATE SET
			   summary_text = EXCLUDED.summary_text, key_decisions = EXCLUDED.key_decisions,
			   risks = EXCLUDED.risks, blockers = EXCLUDED.blockers,
			   model_used = EXCLUDED.model_used, prompt_version = EXCLUDED.prompt_version,
			   created_at = EXCLUDED.created_at
			 WHERE ai.summaries.org_id = $3`,
			s.ID, s.MeetingID, orgID, s.SummaryText, keyDecisions, risks, blockers, s.ModelUsed, s.PromptVersion, s.CreatedAt,
		)
		return err
	})
}

func (r *SummaryRepository) GetSummaryByMeetingID(ctx context.Context, orgID, meetingID string) (*entity.Summary, error) {
	var s entity.Summary
	var keyDecisions, risks, blockers []byte
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT id, meeting_id, org_id, summary_text, key_decisions, risks, blockers, model_used, prompt_version, created_at
			 FROM ai.summaries WHERE meeting_id = $1 AND org_id = $2`,
			meetingID, orgID,
		).Scan(&s.ID, &s.MeetingID, &s.OrgID, &s.SummaryText, &keyDecisions, &risks, &blockers, &s.ModelUsed, &s.PromptVersion, &s.CreatedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("summary not found")
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(keyDecisions, &s.KeyDecisions); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(risks, &s.Risks); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(blockers, &s.Blockers); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SummaryRepository) ReplaceChunks(ctx context.Context, orgID, meetingID string, chunks []*entity.Chunk) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM ai.chunks WHERE meeting_id = $1 AND org_id = $2`, meetingID, orgID); err != nil {
			return err
		}
		if len(chunks) == 0 {
			return nil
		}

		batch := &pgx.Batch{}
		for _, c := range chunks {
			batch.Queue(
				`INSERT INTO ai.chunks (id, meeting_id, org_id, chunk_index, text, token_count, start_ms, end_ms, created_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())`,
				c.ID, c.MeetingID, orgID, c.ChunkIndex, c.Text, c.TokenCount, c.StartMS, c.EndMS,
			)
		}
		br := tx.SendBatch(ctx, batch)
		defer func() { _ = br.Close() }()
		for range chunks {
			if _, err := br.Exec(); err != nil {
				return err
			}
		}
		return nil
	})
}

// Package postgres implements repository.Repository against the
// search.* schema (see docs/architecture/microservices.md §9).
//
// Every query here filters by org_id explicitly in the SQL itself, not
// just through dbx.WithTenantTx's SET LOCAL app.current_org — see
// docs/architecture/database-schema.md's "Row-Level Security pattern"
// section: every service's runtime DB connection is the Postgres
// superuser/table owner, which RLS policies never apply to, so the
// explicit filter is the real enforcement (found the hard way, as a live
// cross-tenant leak, during Phase 2.3 — not repeating that mistake here).
//
// Vectors cross pgx as plain strings, not a registered pgx type: pgvector
// package's Vector.String()/Parse() already produce/consume exactly the
// text format Postgres's `vector` type accepts/emits (e.g.
// "[0.1,0.2,...]"), so every query below writes a vector as
// "$n::vector" bound to that string, and reads one back via an explicit
// "embedding::text" cast — simpler and more explicit than registering a
// custom pgx type for one column, and avoids depending on pgx's
// unregistered-OID fallback behavior.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
)

type SearchRepository struct {
	pool *pgxpool.Pool
}

func NewSearchRepository(pool *pgxpool.Pool) *SearchRepository {
	return &SearchRepository{pool: pool}
}

func (r *SearchRepository) ReplaceEmbeddings(ctx context.Context, orgID, meetingID string, embeddings []*entity.ChunkEmbedding) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM search.chunk_embeddings WHERE meeting_id = $1 AND org_id = $2`, meetingID, orgID); err != nil {
			return err
		}
		if len(embeddings) == 0 {
			return nil
		}

		batch := &pgx.Batch{}
		for _, e := range embeddings {
			batch.Queue(
				`INSERT INTO search.chunk_embeddings
				 (chunk_id, meeting_id, org_id, embedding, model_name, chunk_text, start_ms, end_ms, created_at)
				 VALUES ($1, $2, $3, $4::vector, $5, $6, $7, $8, now())`,
				e.ChunkID, e.MeetingID, orgID, pgvector.NewVector(e.Embedding).String(), e.ModelName, e.MeetingText, e.StartMS, e.EndMS,
			)
		}
		br := tx.SendBatch(ctx, batch)
		defer func() { _ = br.Close() }()
		for range embeddings {
			if _, err := br.Exec(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SearchRepository) GetEmbeddingsByMeeting(ctx context.Context, orgID, meetingID string) ([]*entity.ChunkEmbedding, error) {
	var out []*entity.ChunkEmbedding
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT chunk_id, meeting_id, embedding::text FROM search.chunk_embeddings WHERE meeting_id = $1 AND org_id = $2`,
			meetingID, orgID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e entity.ChunkEmbedding
			var vecText string
			if err := rows.Scan(&e.ChunkID, &e.MeetingID, &vecText); err != nil {
				return err
			}
			var v pgvector.Vector
			if err := v.Parse(vecText); err != nil {
				return err
			}
			e.Embedding = v.Slice()
			out = append(out, &e)
		}
		return rows.Err()
	})
	return out, err
}

// SemanticSearch ranks by pgvector's `<=>` cosine-distance operator
// (lower is better) — see docs/architecture/microservices.md §9's RAG
// flow, which this exact query implements.
func (r *SearchRepository) SemanticSearch(ctx context.Context, orgID string, queryVector []float32, limit int) ([]*entity.SearchHit, error) {
	var hits []*entity.SearchHit
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT chunk_id, meeting_id, chunk_text, start_ms, end_ms, embedding <=> $1::vector AS distance
			 FROM search.chunk_embeddings
			 WHERE org_id = $2
			 ORDER BY distance
			 LIMIT $3`,
			pgvector.NewVector(queryVector).String(), orgID, limit,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var h entity.SearchHit
			if err := rows.Scan(&h.ChunkID, &h.MeetingID, &h.Text, &h.StartMS, &h.EndMS, &h.Score); err != nil {
				return err
			}
			hits = append(hits, &h)
		}
		return rows.Err()
	})
	return hits, err
}

// SimilarMeetings groups every other meeting's chunks by meeting_id and
// ranks by each meeting's single closest-matching chunk to centroid — see
// usecase.SimilarMeetingsUseCase's doc comment for why centroid itself is
// computed in Go rather than via a SQL vector aggregate.
func (r *SearchRepository) SimilarMeetings(ctx context.Context, orgID, excludeMeetingID string, centroid []float32, limit int) ([]*entity.SimilarMeeting, error) {
	var results []*entity.SimilarMeeting
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT meeting_id, MIN(embedding <=> $1::vector) AS best_distance
			 FROM search.chunk_embeddings
			 WHERE org_id = $2 AND meeting_id != $3
			 GROUP BY meeting_id
			 ORDER BY best_distance
			 LIMIT $4`,
			pgvector.NewVector(centroid).String(), orgID, excludeMeetingID, limit,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s entity.SimilarMeeting
			if err := rows.Scan(&s.MeetingID, &s.Score); err != nil {
				return err
			}
			results = append(results, &s)
		}
		return rows.Err()
	})
	return results, err
}

func (r *SearchRepository) SaveQAHistory(ctx context.Context, entry *entity.QAHistoryEntry) error {
	return dbx.WithTenantTx(ctx, r.pool, entry.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO search.qa_history (id, org_id, user_id, question, answer, cited_chunk_ids, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			entry.ID, entry.OrgID, entry.UserID, entry.Question, entry.Answer, entry.CitedChunkIDs, entry.CreatedAt,
		)
		return err
	})
}

func (r *SearchRepository) ListQAHistory(ctx context.Context, orgID, userID string) ([]*entity.QAHistoryEntry, error) {
	var out []*entity.QAHistoryEntry
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id, org_id, user_id, question, answer, cited_chunk_ids, created_at
			 FROM search.qa_history WHERE org_id = $1 AND user_id = $2 ORDER BY created_at DESC`,
			orgID, userID,
		)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var e entity.QAHistoryEntry
			if err := rows.Scan(&e.ID, &e.OrgID, &e.UserID, &e.Question, &e.Answer, &e.CitedChunkIDs, &e.CreatedAt); err != nil {
				return err
			}
			out = append(out, &e)
		}
		return rows.Err()
	})
	return out, err
}

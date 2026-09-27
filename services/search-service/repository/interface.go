// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on *postgres.SearchRepository
// directly, which is what lets it be unit-tested against an in-memory
// fake with zero Postgres involved.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
)

type Repository interface {
	// ReplaceEmbeddings deletes any embeddings already stored for
	// meetingID before inserting the new set — a reindex or a
	// summary/chunk regenerate shouldn't leave stale vectors from a
	// previous pass lingering alongside the fresh ones.
	ReplaceEmbeddings(ctx context.Context, orgID, meetingID string, embeddings []*entity.ChunkEmbedding) error

	// GetEmbeddingsByMeeting feeds SimilarMeetingsUseCase's centroid
	// computation (see that usecase's doc comment for why the average is
	// computed in Go rather than via a `avg(vector)` SQL aggregate).
	GetEmbeddingsByMeeting(ctx context.Context, orgID, meetingID string) ([]*entity.ChunkEmbedding, error)

	// SemanticSearch returns the top `limit` chunks by cosine distance to
	// queryVector, org-scoped. Used by both GET /search and POST /qa/ask
	// (RAG retrieval is just search with a different consumer of the
	// results).
	SemanticSearch(ctx context.Context, orgID string, queryVector []float32, limit int) ([]*entity.SearchHit, error)

	// SimilarMeetings ranks every other meeting in the org by its best
	// (minimum) cosine distance to centroid, excluding excludeMeetingID
	// itself.
	SimilarMeetings(ctx context.Context, orgID, excludeMeetingID string, centroid []float32, limit int) ([]*entity.SimilarMeeting, error)

	SaveQAHistory(ctx context.Context, entry *entity.QAHistoryEntry) error
	ListQAHistory(ctx context.Context, orgID, userID string) ([]*entity.QAHistoryEntry, error)
}

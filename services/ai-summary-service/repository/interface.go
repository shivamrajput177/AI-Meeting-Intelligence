// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on *postgres.SummaryRepository
// directly, which is what lets it be unit-tested against an in-memory
// fake with zero Postgres involved.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
)

type Repository interface {
	// UpsertSummary replaces any existing summary for the same
	// meeting_id (POST .../summary/regenerate calls this exactly like the
	// Kafka-consumer path does — one summary per meeting, always the
	// latest run).
	UpsertSummary(ctx context.Context, orgID string, s *entity.Summary) error
	GetSummaryByMeetingID(ctx context.Context, orgID, meetingID string) (*entity.Summary, error)

	// ReplaceChunks deletes any chunks already stored for meetingID before
	// inserting the new set — a regenerate run shouldn't leave stale
	// chunks from a previous pass lingering alongside the fresh ones.
	ReplaceChunks(ctx context.Context, orgID, meetingID string, chunks []*entity.Chunk) error
}

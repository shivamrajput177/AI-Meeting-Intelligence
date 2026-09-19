// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on *postgres.TranscriptRepository
// directly, which is what lets it be unit-tested against an in-memory
// fake with zero Postgres involved.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
)

type Repository interface {
	CreateTranscript(ctx context.Context, t *entity.Transcript) error
	CreateSegments(ctx context.Context, orgID string, segments []*entity.Segment) error
	GetByMeetingID(ctx context.Context, orgID, meetingID string) (*entity.Transcript, []*entity.Segment, error)
}

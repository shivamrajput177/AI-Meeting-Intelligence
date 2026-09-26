// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on *postgres.MeetingRepository
// directly, which is what lets it be unit-tested against an in-memory
// fake with zero Postgres involved.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
)

type Repository interface {
	Create(ctx context.Context, m *entity.Meeting) error
	GetByID(ctx context.Context, orgID, id string) (*entity.Meeting, error)
	List(ctx context.Context, orgID string, filter entity.ListFilter) (items []*entity.Meeting, total int, err error)
	UpdateStatus(ctx context.Context, orgID, id, status string) error
	Touch(ctx context.Context, orgID, id string) error
	Delete(ctx context.Context, orgID, id string) error

	// ListParticipants is org-scoped via a join against meeting.meetings —
	// meeting.participants itself has no org_id column (see
	// migrations/0001_init.up.sql), so the join is the enforcement here,
	// same explicit-filter discipline as every other query in this file.
	ListParticipants(ctx context.Context, orgID, meetingID string) ([]*entity.Participant, error)
}

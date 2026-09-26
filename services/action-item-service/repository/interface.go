// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on *postgres.ActionItemRepository
// directly, which is what lets it be unit-tested against an in-memory
// fake with zero Postgres involved.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

type Repository interface {
	// ReplaceActionItems deletes any action items already stored for
	// meetingID before inserting the new set — re-extraction (a summary
	// regenerate re-publishing summary.completed.v1) shouldn't leave
	// stale items from a previous pass alongside the fresh ones, same
	// trade-off ai-summary-service's ReplaceChunks already makes.
	ReplaceActionItems(ctx context.Context, orgID, meetingID string, items []*entity.ActionItem) error

	GetByID(ctx context.Context, orgID, id string) (*entity.ActionItem, error)
	ListByMeeting(ctx context.Context, orgID, meetingID string) ([]*entity.ActionItem, error)
	List(ctx context.Context, orgID string, filter entity.ListActionItemsFilter) (items []*entity.ActionItem, total int, err error)
	Update(ctx context.Context, orgID, id string, input entity.UpdateActionItemInput) (*entity.ActionItem, error)
}

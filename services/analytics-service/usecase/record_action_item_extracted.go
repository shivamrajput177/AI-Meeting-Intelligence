package usecase

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/actionitems"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/repository"
)

// RecordActionItemsExtractedUseCase rolls action-item.extracted.v1 events
// into analytics.action_item_rollup's "opened" counts — see
// docs/architecture/microservices.md §11.
type RecordActionItemsExtractedUseCase struct {
	actionItems actionitems.Client
	repo        repository.Repository
}

func NewRecordActionItemsExtractedUseCase(actionItems actionitems.Client, repo repository.Repository) *RecordActionItemsExtractedUseCase {
	return &RecordActionItemsExtractedUseCase{actionItems, repo}
}

// RecordActionItemsExtracted looks up meetingID's own items (the
// extraction event itself only carries a batch count, see
// entity.ActionItemExtractedEvent) and increments each owned item's
// (org, owner, day) "opened" count by one. Items with no matched owner are
// skipped — see entity.ActionItemRollup's doc comment on why there's no
// "unassigned" bucket for this table. The first per-item error is
// returned after every item has been attempted, not on the first failure,
// so one bad increment doesn't stop the rest of the same meeting's items
// from being counted.
func (uc *RecordActionItemsExtractedUseCase) RecordActionItemsExtracted(ctx context.Context, orgID, meetingID string, eventTime time.Time) error {
	items, err := uc.actionItems.ListForMeeting(ctx, orgID, meetingID)
	if err != nil {
		return err
	}
	day := dayOf(eventTime)

	var firstErr error
	for _, it := range items {
		if it.OwnerUserID == nil {
			continue
		}
		if err := uc.repo.IncrementActionItemOpened(ctx, orgID, *it.OwnerUserID, day); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

package usecase

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/repository"
)

// actionItemStatusDone mirrors actionitemsvc/entity.StatusDone —
// duplicated, not imported, matching every other cross-service constant
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
const actionItemStatusDone = "done"

// RecordActionItemStatusChangedUseCase rolls action-item.status-changed.v1
// events into analytics.action_item_rollup's "closed" counts — see
// docs/architecture/microservices.md §11.
type RecordActionItemStatusChangedUseCase struct {
	repo repository.Repository
}

func NewRecordActionItemStatusChangedUseCase(repo repository.Repository) *RecordActionItemStatusChangedUseCase {
	return &RecordActionItemStatusChangedUseCase{repo}
}

// RecordActionItemStatusChanged only counts a transition to "done" as
// closed — the topic also fires for "in_progress"/"cancelled"/reopening
// back to "open" (see actionitemsvc's UpdateActionItemUseCase), none of
// which this rollup's "closed" column is meant to count. An item with no
// owner is skipped, same reasoning as the opened-rollup.
func (uc *RecordActionItemStatusChangedUseCase) RecordActionItemStatusChanged(ctx context.Context, orgID string, ownerUserID *string, status string, eventTime time.Time) error {
	if status != actionItemStatusDone || ownerUserID == nil {
		return nil
	}
	return uc.repo.IncrementActionItemClosed(ctx, orgID, *ownerUserID, dayOf(eventTime))
}

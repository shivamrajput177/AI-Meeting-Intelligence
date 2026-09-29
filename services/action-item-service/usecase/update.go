package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

type UpdateActionItemUseCase struct {
	repo      repository.Repository
	publisher events.Publisher
	log       *logger.Logger
}

func NewUpdateActionItemUseCase(repo repository.Repository, publisher events.Publisher, log *logger.Logger) *UpdateActionItemUseCase {
	return &UpdateActionItemUseCase{repo, publisher, log}
}

// UpdateActionItem is PATCH /action-items/{id}'s business logic —
// docs/architecture/api-spec.md gates this route "member+ (owner or
// admin)", a per-resource check the gateway's role-only RequireRole can't
// express (it doesn't know who a given item is assigned to), so it's
// re-checked here: an org owner/admin may update any item, anyone else
// only the one item already assigned to them.
func (uc *UpdateActionItemUseCase) UpdateActionItem(
	ctx context.Context, orgID, id, callerUserID, callerRole string, input entity.UpdateActionItemInput,
) (*entity.ActionItem, error) {
	item, err := uc.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	isPrivileged := callerRole == "owner" || callerRole == "admin"
	isAssignedOwner := item.OwnerUserID != nil && *item.OwnerUserID == callerUserID
	if !isPrivileged && !isAssignedOwner {
		return nil, apperr.Forbidden("only the item's owner or an org owner/admin can update it")
	}
	statusChanged := input.Status != nil && *input.Status != item.Status

	updated, err := uc.repo.Update(ctx, orgID, id, input)
	if err != nil {
		return nil, err
	}

	// action-item.status-changed.v1 is Analytics Service's signal for its
	// productivity/completion-rate rollups — only published when a PATCH
	// actually changed status (not on an owner/due-date-only update),
	// matching this topic's documented "owner marks done/in-progress"
	// purpose. A publish failure is logged, not returned: the PATCH itself
	// already succeeded and the caller shouldn't see a 500 for a
	// side-channel analytics signal (same tradeoff meeting-service's
	// UpdateStatusUseCase makes for meeting.status-changed.v1).
	if statusChanged {
		event := entity.ActionItemStatusChangedEvent{
			ActionItemID: updated.ID, MeetingID: updated.MeetingID, OrgID: orgID,
			OwnerUserID: updated.OwnerUserID, Status: updated.Status,
		}
		if err := uc.publisher.PublishActionItemStatusChanged(ctx, event); err != nil {
			uc.log.Error("publish action-item.status-changed.v1", "err", err, "actionItemId", updated.ID)
		}
	}

	return updated, nil
}

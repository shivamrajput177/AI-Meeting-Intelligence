package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type UpdateActionItemUseCase struct{ repo repository.Repository }

func NewUpdateActionItemUseCase(repo repository.Repository) *UpdateActionItemUseCase {
	return &UpdateActionItemUseCase{repo}
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
	return uc.repo.Update(ctx, orgID, id, input)
}

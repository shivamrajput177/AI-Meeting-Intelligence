package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository"
)

// UpdateActionItemInternalUseCase is PATCH /internal/action-items/{id}'s
// business logic — a trusted, system-initiated write with no
// owner/admin/assigned-owner authorization check, unlike
// UpdateActionItemUseCase's public PATCH: the only caller is Notification
// Service, writing back a Jira ticket key after POST
// .../jira-ticket, or a status change the mock Jira board's own card drag
// maps onto (see deployment-demo-strategy.md §3's "genuinely
// bidirectional" design).
type UpdateActionItemInternalUseCase struct {
	repo repository.Repository
}

func NewUpdateActionItemInternalUseCase(repo repository.Repository) *UpdateActionItemInternalUseCase {
	return &UpdateActionItemInternalUseCase{repo}
}

func (uc *UpdateActionItemInternalUseCase) UpdateActionItemInternal(ctx context.Context, orgID, id string, input entity.UpdateActionItemInput) (*entity.ActionItem, error) {
	return uc.repo.Update(ctx, orgID, id, input)
}

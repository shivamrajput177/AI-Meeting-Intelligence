package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

// UpdateIntegrationConfigUseCase is PATCH /orgs/{orgId}/settings' business
// logic — see entity.UpdateIntegrationConfigRequest's own doc comment on
// why this is scoped to integration config only, not general org
// settings.
type UpdateIntegrationConfigUseCase struct {
	repo repository.IntegrationConfigRepository
}

func NewUpdateIntegrationConfigUseCase(repo repository.IntegrationConfigRepository) *UpdateIntegrationConfigUseCase {
	return &UpdateIntegrationConfigUseCase{repo: repo}
}

func (uc *UpdateIntegrationConfigUseCase) UpdateIntegrationConfig(ctx context.Context, orgID string, input entity.UpdateIntegrationConfigInput) (*entity.IntegrationConfig, error) {
	if input.TicketProvider != nil && !entity.ValidTicketProviders[*input.TicketProvider] {
		return nil, apperr.BadRequest("invalid ticketProvider")
	}
	return uc.repo.UpdateIntegrationConfig(ctx, orgID, input)
}

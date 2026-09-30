package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/repository"
)

// GetIntegrationConfigUseCase backs both GET
// /internal/orgs/{orgId}/integration-config (Notification Service's own
// read, at dispatch time) and PATCH /orgs/{orgId}/settings' response
// (returning the settings as they now stand after an update).
type GetIntegrationConfigUseCase struct {
	repo repository.IntegrationConfigRepository
}

func NewGetIntegrationConfigUseCase(repo repository.IntegrationConfigRepository) *GetIntegrationConfigUseCase {
	return &GetIntegrationConfigUseCase{repo: repo}
}

func (uc *GetIntegrationConfigUseCase) GetIntegrationConfig(ctx context.Context, orgID string) (*entity.IntegrationConfig, error) {
	return uc.repo.GetIntegrationConfig(ctx, orgID)
}

package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/domain"
)

type GetOrgUseCase struct {
	repo domain.Repository
}

func NewGetOrgUseCase(repo domain.Repository) *GetOrgUseCase {
	return &GetOrgUseCase{repo: repo}
}

func (uc *GetOrgUseCase) GetOrg(ctx context.Context, id string) (*domain.Organization, error) {
	return uc.repo.GetByID(ctx, id)
}

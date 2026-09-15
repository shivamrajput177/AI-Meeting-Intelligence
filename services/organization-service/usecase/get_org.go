package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/repository"
)

type GetOrgUseCase struct {
	repo repository.Repository
}

func NewGetOrgUseCase(repo repository.Repository) *GetOrgUseCase {
	return &GetOrgUseCase{repo: repo}
}

func (uc *GetOrgUseCase) GetOrg(ctx context.Context, id string) (*entity.Organization, error) {
	return uc.repo.GetByID(ctx, id)
}

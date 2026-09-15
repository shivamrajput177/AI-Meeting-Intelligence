package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type DeactivateUserUseCase struct {
	repo repository.Repository
}

func NewDeactivateUserUseCase(repo repository.Repository) *DeactivateUserUseCase {
	return &DeactivateUserUseCase{repo: repo}
}

// DeactivateUser flips a user to StatusDeactivated (soft delete — the row
// stays, everything it's the foreign key for stays intact) rather than
// actually deleting it. The org's owner can't be deactivated through this
// endpoint, nor can a caller deactivate themselves — either would leave
// the org with nobody able to undo it.
func (uc *DeactivateUserUseCase) DeactivateUser(ctx context.Context, orgID, userID, callerUserID string) (*entity.User, error) {
	if userID == callerUserID {
		return nil, apperr.BadRequest("cannot deactivate your own account")
	}

	target, err := uc.repo.GetByID(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	if target.Role == entity.RoleOwner {
		return nil, apperr.Forbidden("cannot deactivate the organization owner")
	}

	return uc.repo.Deactivate(ctx, orgID, userID)
}

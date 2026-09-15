package usecase

import (
	"context"
	"slices"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type UpdateRoleUseCase struct {
	repo repository.Repository
}

func NewUpdateRoleUseCase(repo repository.Repository) *UpdateRoleUseCase {
	return &UpdateRoleUseCase{repo: repo}
}

// UpdateRole changes a user's RBAC role. The org's owner is immutable
// through this endpoint (both as a target and as a value) — ownership
// transfer would be its own, more careful flow — and an actor can't
// change their own role, to rule out a caller accidentally locking
// themselves out of the one action that would undo it.
func (uc *UpdateRoleUseCase) UpdateRole(ctx context.Context, in entity.UpdateRoleInput) (*entity.User, error) {
	if !slices.Contains(invitableRoles, in.Role) {
		return nil, apperr.BadRequest("role must be one of admin, manager, member, viewer")
	}
	if in.UserID == in.CallerUserID {
		return nil, apperr.BadRequest("cannot change your own role")
	}

	target, err := uc.repo.GetByID(ctx, in.OrgID, in.UserID)
	if err != nil {
		return nil, err
	}
	if target.Role == entity.RoleOwner {
		return nil, apperr.Forbidden("cannot change the organization owner's role")
	}

	return uc.repo.UpdateRole(ctx, in.OrgID, in.UserID, in.Role)
}

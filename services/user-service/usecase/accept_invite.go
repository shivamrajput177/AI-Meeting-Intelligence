package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/jwtutil"
)

// AcceptInviteUseCase resolves an invite token into a brand-new active
// user row — email and role come from the invite an owner/admin already
// created, name from what the invitee supplies at accept time. Called
// only over /internal/invites/accept by Auth Service's own AcceptInvite
// usecase, which still has to create the credentials row and issue
// tokens afterward (User Service owns no password/JWT concern at all —
// see docs/architecture/microservices.md §2/§3).
type AcceptInviteUseCase struct {
	repo repository.Repository
}

func NewAcceptInviteUseCase(repo repository.Repository) *AcceptInviteUseCase {
	return &AcceptInviteUseCase{repo: repo}
}

func (uc *AcceptInviteUseCase) AcceptInvite(ctx context.Context, token, name string) (*entity.User, error) {
	name = strings.TrimSpace(name)
	if token == "" || name == "" {
		return nil, apperr.BadRequest("token and name are required")
	}

	invite, err := uc.repo.GetInviteByTokenHash(ctx, jwtutil.HashRefreshToken(token))
	if err != nil {
		return nil, err
	}
	if invite.AcceptedAt != nil || invite.ExpiresAt.Before(time.Now()) {
		return nil, apperr.BadRequest("invalid or expired invite token")
	}

	user := &entity.User{
		ID:        uuid.NewString(),
		OrgID:     invite.OrgID,
		Email:     invite.Email,
		Name:      name,
		Role:      invite.Role,
		Status:    entity.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	if err := uc.repo.MarkInviteAccepted(ctx, invite.OrgID, invite.ID); err != nil {
		return nil, apperr.Internal("mark invite accepted").Wrap(err)
	}
	return user, nil
}

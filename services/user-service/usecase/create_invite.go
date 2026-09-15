package usecase

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/jwtutil"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// inviteTokenTTL mirrors auth-service's resetTokenTTL in shape (a fixed,
// generous window an invite link stays valid for) but is longer — an
// invite is something a person might not see for a few days, unlike a
// reset link someone requested moments ago.
const inviteTokenTTL = 7 * 24 * time.Hour

// invitableRoles excludes RoleOwner deliberately — ownership isn't
// something an invite grants; transferring it would be its own, more
// careful flow, out of Phase 2's scope.
var invitableRoles = []string{entity.RoleAdmin, entity.RoleManager, entity.RoleMember, entity.RoleViewer}

// CreateInviteUseCase creates a pending org invite. There is no
// Notification Service yet to email it (that's Phase 4 — see
// docs/architecture/microservices.md §10), so — exactly like
// auth-service's RequestPasswordResetUseCase — Phase 2 logs the raw token
// server-side and, only when devExposeToken is true (an explicit
// local-dev flag, never set in the public demo deployment), returns it in
// the response so the invite flow is testable end-to-end without a real
// mailbox.
type CreateInviteUseCase struct {
	repo           repository.Repository
	log            *logger.Logger
	devExposeToken bool
}

func NewCreateInviteUseCase(repo repository.Repository, log *logger.Logger, devExposeToken bool) *CreateInviteUseCase {
	return &CreateInviteUseCase{repo: repo, log: log, devExposeToken: devExposeToken}
}

func (uc *CreateInviteUseCase) CreateInvite(ctx context.Context, in entity.CreateInviteInput) (invite *entity.Invite, devToken string, err error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" {
		return nil, "", apperr.BadRequest("email is required")
	}
	if !slices.Contains(invitableRoles, in.Role) {
		return nil, "", apperr.BadRequest("role must be one of admin, manager, member, viewer")
	}

	raw, hash, genErr := jwtutil.NewOpaqueRefreshToken()
	if genErr != nil {
		return nil, "", apperr.Internal("generate invite token").Wrap(genErr)
	}

	invite = &entity.Invite{
		ID:        uuid.NewString(),
		OrgID:     in.OrgID,
		Email:     email,
		Role:      in.Role,
		TokenHash: hash,
		InvitedBy: in.InvitedBy,
		ExpiresAt: time.Now().Add(inviteTokenTTL),
	}
	if err := uc.repo.CreateInvite(ctx, invite); err != nil {
		return nil, "", err
	}

	uc.log.Info("invite created", "org_id", in.OrgID, "email", email, "role", in.Role, "dev_invite_token", raw)

	if uc.devExposeToken {
		return invite, raw, nil
	}
	return invite, "", nil
}

package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/client"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/passwordutil"
)

// AcceptInviteUseCase is the invite-flow counterpart to Signup: instead
// of creating a brand-new org, it joins the invitee to the org and role
// an owner/admin already chose when creating the invite (User Service's
// own CreateInviteUseCase), then issues tokens exactly like
// signup/login do so the invitee lands signed in immediately.
type AcceptInviteUseCase struct {
	userClient  client.UserClient
	credentials repository.CredentialsRepository
	tokenIssuer *TokenIssuer
}

func NewAcceptInviteUseCase(userClient client.UserClient, credentials repository.CredentialsRepository, tokenIssuer *TokenIssuer) *AcceptInviteUseCase {
	return &AcceptInviteUseCase{userClient: userClient, credentials: credentials, tokenIssuer: tokenIssuer}
}

func (uc *AcceptInviteUseCase) AcceptInvite(ctx context.Context, in entity.AcceptInviteInput) (*entity.TokenPair, error) {
	token := strings.TrimSpace(in.Token)
	name := strings.TrimSpace(in.Name)
	if token == "" || name == "" {
		return nil, apperr.BadRequest("token and name are required")
	}
	if len(in.Password) < 8 {
		return nil, apperr.BadRequest("password must be at least 8 characters")
	}

	userID, orgID, role, _, err := uc.userClient.AcceptInvite(ctx, token, name)
	if err != nil {
		return nil, err
	}

	hash, err := passwordutil.Hash(in.Password)
	if err != nil {
		return nil, apperr.Internal("hash password").Wrap(err)
	}
	if err := uc.credentials.Create(ctx, &entity.Credentials{
		UserID: userID, OrgID: orgID, PasswordHash: hash, Algo: "argon2id", UpdatedAt: time.Now(),
	}); err != nil {
		return nil, apperr.Internal("store credentials").Wrap(err)
	}

	return uc.tokenIssuer.Issue(ctx, userID, orgID, role)
}

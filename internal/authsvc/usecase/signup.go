package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/passwordutil"
)

type SignupInput struct {
	OrgName  string
	Email    string
	Name     string
	Password string
}

type SignupUseCase struct {
	orgClient   domain.OrgClient
	userClient  domain.UserClient
	credentials domain.CredentialsRepository
	tokenIssuer *TokenIssuer
}

func NewSignupUseCase(orgClient domain.OrgClient, userClient domain.UserClient, credentials domain.CredentialsRepository, tokenIssuer *TokenIssuer) *SignupUseCase {
	return &SignupUseCase{orgClient: orgClient, userClient: userClient, credentials: credentials, tokenIssuer: tokenIssuer}
}

// Execute creates a brand-new organization and its owner user, then
// issues tokens for them — the only signup path in Phase 1 (see
// docs/ROADMAP.md Phase 1: "signup creates one org and its owner"). There
// is deliberately no "join an existing org" path yet; that's the invite
// flow, Phase 2.
func (uc *SignupUseCase) Execute(ctx context.Context, in SignupInput) (*TokenPair, error) {
	orgName := strings.TrimSpace(in.OrgName)
	email := strings.TrimSpace(strings.ToLower(in.Email))
	name := strings.TrimSpace(in.Name)
	if orgName == "" || email == "" || name == "" {
		return nil, apperr.BadRequest("orgName, email, and name are required")
	}
	if len(in.Password) < 8 {
		return nil, apperr.BadRequest("password must be at least 8 characters")
	}

	orgID, err := uc.orgClient.CreateOrg(ctx, orgName)
	if err != nil {
		return nil, err
	}

	userID, err := uc.userClient.CreateUser(ctx, orgID, email, name, domain.RoleOwner)
	if err != nil {
		return nil, err
	}

	hash, err := passwordutil.Hash(in.Password)
	if err != nil {
		return nil, apperr.Internal("hash password").Wrap(err)
	}
	if err := uc.credentials.Create(ctx, &domain.Credentials{
		UserID: userID, OrgID: orgID, PasswordHash: hash, Algo: "argon2id", UpdatedAt: time.Now(),
	}); err != nil {
		return nil, apperr.Internal("store credentials").Wrap(err)
	}

	return uc.tokenIssuer.Issue(ctx, userID, orgID, domain.RoleOwner)
}

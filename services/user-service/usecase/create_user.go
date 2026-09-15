package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type CreateUserUseCase struct {
	repo repository.Repository
}

func NewCreateUserUseCase(repo repository.Repository) *CreateUserUseCase {
	return &CreateUserUseCase{repo: repo}
}

// CreateUser creates the user row for a brand-new signup (called by Auth
// Service over the /internal/users route — see
// docs/architecture/microservices.md §3, "User row creation itself isn't
// a called endpoint" is the Phase 2+ event-driven version of this; Phase 1
// calls it synchronously since Kafka doesn't exist yet).
func (uc *CreateUserUseCase) CreateUser(ctx context.Context, in entity.CreateUserInput) (*entity.User, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	name := strings.TrimSpace(in.Name)
	if email == "" || name == "" {
		return nil, apperr.BadRequest("email and name are required")
	}
	if in.Role != entity.RoleOwner && in.Role != entity.RoleMember {
		// Phase 1 only ever creates an owner (signup) or a member — the
		// admin/manager/viewer roles are assignable starting Phase 2's
		// PATCH .../role, once RBAC actually exists.
		return nil, apperr.BadRequest("role must be owner or member in Phase 1")
	}

	user := &entity.User{
		ID:        uuid.NewString(),
		OrgID:     in.OrgID,
		Email:     email,
		Name:      name,
		Role:      in.Role,
		Status:    entity.StatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

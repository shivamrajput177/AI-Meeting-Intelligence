package usecase

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type CreateOrgUseCase struct {
	repo domain.Repository
}

func NewCreateOrgUseCase(repo domain.Repository) *CreateOrgUseCase {
	return &CreateOrgUseCase{repo: repo}
}

func (uc *CreateOrgUseCase) CreateOrg(ctx context.Context, in entity.CreateOrgInput) (*domain.Organization, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, apperr.BadRequest("name is required")
	}

	org := &domain.Organization{
		ID:        uuid.NewString(),
		Name:      name,
		Slug:      slugify(name) + "-" + uuid.NewString()[:8],
		Plan:      "free",
		Status:    "active",
		CreatedAt: time.Now(),
	}
	if err := uc.repo.Create(ctx, org); err != nil {
		return nil, apperr.Internal("create organization").Wrap(err)
	}
	return org, nil
}

var slugInvalidChars = regexp.MustCompile(`[^a-z0-9]+`)

// slugify is intentionally simple — the uuid suffix Execute appends is
// what actually guarantees uniqueness, this just makes the slug readable.
func slugify(name string) string {
	s := slugInvalidChars.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(s, "-")
}

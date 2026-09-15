package usecase

import (
	"context"
	"strings"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/repository"
)

type LookupByEmailUseCase struct {
	repo repository.Repository
}

func NewLookupByEmailUseCase(repo repository.Repository) *LookupByEmailUseCase {
	return &LookupByEmailUseCase{repo: repo}
}

// LookupByEmail is used only by Auth Service's login flow to resolve which
// org(s) an email belongs to before it knows the org_id. See
// entity.EmailLookup's doc comment for what this deliberately does and
// does not expose.
func (uc *LookupByEmailUseCase) LookupByEmail(ctx context.Context, email string) ([]entity.EmailLookup, error) {
	return uc.repo.LookupByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
}

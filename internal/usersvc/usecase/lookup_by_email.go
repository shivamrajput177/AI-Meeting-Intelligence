package usecase

import (
	"context"
	"strings"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/usersvc/domain"
)

type LookupByEmailUseCase struct {
	repo domain.Repository
}

func NewLookupByEmailUseCase(repo domain.Repository) *LookupByEmailUseCase {
	return &LookupByEmailUseCase{repo: repo}
}

// Execute is used only by Auth Service's login flow to resolve which
// org(s) an email belongs to before it knows the org_id. See
// domain.EmailLookup's doc comment for what this deliberately does and
// does not expose.
func (uc *LookupByEmailUseCase) Execute(ctx context.Context, email string) ([]domain.EmailLookup, error) {
	return uc.repo.LookupByEmail(ctx, strings.TrimSpace(strings.ToLower(email)))
}

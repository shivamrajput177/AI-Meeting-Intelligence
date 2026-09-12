package usecase_test

import (
	"context"
	"sync"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/usecase"
)

// fakeRepository is an in-memory stand-in for the Postgres implementation
// — this is the whole point of defining domain.Repository as an
// interface the usecase depends on (see
// docs/architecture/folder-structure.md's Clean Architecture layering):
// CreateOrgUseCase is fully testable with no real Postgres involved.
type fakeRepository struct {
	mu   sync.Mutex
	orgs map[string]*domain.Organization
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{orgs: make(map[string]*domain.Organization)}
}

func (f *fakeRepository) Create(_ context.Context, org *domain.Organization) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.orgs[org.ID] = org
	return nil
}

func (f *fakeRepository) GetByID(_ context.Context, id string) (*domain.Organization, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	org, ok := f.orgs[id]
	if !ok {
		return nil, domain.ErrOrgNotFound
	}
	return org, nil
}

func TestCreateOrgUseCase_Execute(t *testing.T) {
	repo := newFakeRepository()
	uc := usecase.NewCreateOrgUseCase(repo)

	org, err := uc.Execute(context.Background(), usecase.CreateOrgInput{Name: "Acme Inc"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if org.Name != "Acme Inc" {
		t.Fatalf("Name = %q, want %q", org.Name, "Acme Inc")
	}
	if org.Plan != "free" || org.Status != "active" {
		t.Fatalf("unexpected defaults: plan=%q status=%q", org.Plan, org.Status)
	}
	if org.Slug == "" {
		t.Fatal("expected a non-empty slug")
	}

	fetched, err := uc.Execute(context.Background(), usecase.CreateOrgInput{Name: "Acme Inc"})
	if err != nil {
		t.Fatalf("Execute (second org): %v", err)
	}
	if fetched.Slug == org.Slug {
		t.Fatal("expected two orgs with the same name to still get distinct slugs")
	}
}

func TestCreateOrgUseCase_Execute_RejectsEmptyName(t *testing.T) {
	uc := usecase.NewCreateOrgUseCase(newFakeRepository())

	if _, err := uc.Execute(context.Background(), usecase.CreateOrgInput{Name: "   "}); err == nil {
		t.Fatal("expected an error for a blank org name")
	}
}

func TestGetOrgUseCase_Execute_NotFound(t *testing.T) {
	uc := usecase.NewGetOrgUseCase(newFakeRepository())

	if _, err := uc.Execute(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected domain.ErrOrgNotFound for an unknown id")
	}
}

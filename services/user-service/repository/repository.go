// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on *postgres.UserRepository
// directly, which is what lets it be unit-tested against an in-memory
// fake with zero Postgres involved.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
)

type Repository interface {
	Create(ctx context.Context, user *entity.User) error
	GetByID(ctx context.Context, orgID, userID string) (*entity.User, error)
	UpdateProfile(ctx context.Context, orgID, userID, name, avatarURL string) (*entity.User, error)

	// LookupByEmail intentionally bypasses tenant context (see
	// repository/postgres for why) — callers must treat its result as
	// sensitive-adjacent (it reveals which orgs an email belongs to) and
	// it must never be exposed through the public API, only over
	// /internal/* to Auth Service.
	LookupByEmail(ctx context.Context, email string) ([]entity.EmailLookup, error)
}

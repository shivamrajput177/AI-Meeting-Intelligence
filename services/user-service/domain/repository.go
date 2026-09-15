package domain

import "context"

// EmailLookup is the minimal, non-sensitive projection returned by
// LookupByEmail — just enough for Auth Service's login flow to resolve
// which user+org a bare email address means, without RLS/tenant context
// (deliberately: at login time, the caller doesn't know their org_id yet
// — that's the thing being resolved). See LookupByEmail's doc comment for
// the multi-org-per-email trade-off this accepts in Phase 1.
type EmailLookup struct {
	UserID string
	OrgID  string
	Role   string
	Status string
}

type Repository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, orgID, userID string) (*User, error)
	UpdateProfile(ctx context.Context, orgID, userID, name, avatarURL string) (*User, error)

	// LookupByEmail intentionally bypasses tenant context (see
	// repository/postgres for why) — callers must treat its result as
	// sensitive-adjacent (it reveals which orgs an email belongs to) and
	// it must never be exposed through the public API, only over
	// /internal/* to Auth Service.
	LookupByEmail(ctx context.Context, email string) ([]EmailLookup, error)
}

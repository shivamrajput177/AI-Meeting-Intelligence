package domain

import "context"

// OrgClient and UserClient are the ports Auth Service's signup usecase
// calls through to create the org+owner rows that actually live in
// Organization/User Service's own schemas. Auth Service owns neither
// table — see docs/architecture/microservices.md §2's schema list — so
// signup necessarily orchestrates two other services' REST APIs rather
// than writing to their tables directly.
//
// Known limitation, called out rather than hidden: this is a multi-step
// operation with no distributed transaction across three services
// (Org, User, Auth's own credentials insert). If User Service's call
// succeeds but Auth's own credentials.Create fails, the result is an org
// and a user with no way to log in. Phase 1 accepts this (it's a rare
// failure mode and the alternative — a saga/compensating-transaction
// flow — is real complexity this walking skeleton doesn't need yet); it's
// exactly the kind of thing docs/ROADMAP.md flags as a good interview
// topic ("saga-like multi-step async workflows"), worth naming explicitly
// rather than pretending it isn't a gap.
type OrgClient interface {
	CreateOrg(ctx context.Context, name string) (orgID string, err error)
}

type UserClient interface {
	CreateUser(ctx context.Context, orgID, email, name, role string) (userID string, err error)
	LookupByEmail(ctx context.Context, email string) ([]EmailMatch, error)
}

// EmailMatch mirrors usersvc/domain.EmailLookup — duplicated rather than
// imported so Auth Service's domain package has zero dependency on User
// Service's internals, matching every other service boundary in this repo.
type EmailMatch struct {
	UserID string
	OrgID  string
	Role   string
	Status string
}

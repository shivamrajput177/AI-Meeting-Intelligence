// Package client defines the OrgClient/UserClient ports Auth Service's
// signup usecase calls through to create the org+owner rows that
// actually live in Organization/User Service's own schemas. Auth
// Service owns neither table — see docs/architecture/microservices.md
// §2's schema list — so signup necessarily orchestrates two other
// services' REST APIs rather than writing to their tables directly.
// client/http, right below this package in the same tree, implements
// both against the real internal REST APIs — keeping the interfaces
// here instead of off in some unrelated package is just where they
// belong, next to their one real implementation.
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
package client

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
)

type OrgClient interface {
	CreateOrg(ctx context.Context, name string) (orgID string, err error)
}

type UserClient interface {
	CreateUser(ctx context.Context, orgID, email, name, role string) (userID string, err error)
	LookupByEmail(ctx context.Context, email string) ([]entity.EmailMatch, error)

	// AcceptInvite validates an invite token against User Service, which
	// creates the invited user's row (email/role come from the invite
	// itself, name from what the invitee supplies at accept time) — the
	// invite-flow counterpart to CreateUser's role in signup. See
	// docs/architecture/api-spec.md §Users, POST /invites/{token}/accept.
	AcceptInvite(ctx context.Context, token, name string) (userID, orgID, role, email string, err error)
}

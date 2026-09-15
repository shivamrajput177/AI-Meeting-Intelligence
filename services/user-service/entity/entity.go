// Package entity holds User Service's plain data structs: its own
// internal model (User — what repository reads out of Postgres and
// usecase operates on), the JSON wire shapes at this service's REST
// boundary, and the plain input structs its usecase layer passes around
// internally. Nothing here has behavior (no methods, just fields, and
// json tags where the struct crosses the wire): it's data, not a class,
// which is what keeps it out of handler/ and usecase/ — those packages
// hold the code that does something with an entity, this package only
// describes its shape.
package entity

import "time"

// Role values. Phase 1 only ever assigns RoleOwner (the signup user) or
// RoleMember (everyone else, though nothing creates a second user yet —
// that's the invite flow, deferred to Phase 2). All five values already
// exist in the CHECK constraint so enabling the rest is additive, not a
// migration — see docs/ROADMAP.md Phase 1/2 and
// docs/architecture/api-spec.md §Users.
const (
	RoleOwner   = "owner"
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleMember  = "member"
	RoleViewer  = "viewer"
)

const (
	StatusInvited     = "invited"
	StatusActive      = "active"
	StatusDeactivated = "deactivated"
)

// User is this service's own internal model — what repository reads out
// of Postgres and usecase operates on. Not JSON-tagged: it never crosses
// the wire directly, handler.go always reshapes it into a UserResponse
// first (see toUserResponse).
type User struct {
	ID        string
	OrgID     string
	Email     string
	Name      string
	Role      string
	Status    string
	AvatarURL string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Invite is this service's own internal model for a pending org invite —
// what repository reads out of/writes to Postgres. Not JSON-tagged: it
// never crosses the wire directly (CreateInvite always reshapes it into
// an InviteResponse).
type Invite struct {
	ID         string
	OrgID      string
	Email      string
	Role       string
	TokenHash  string
	InvitedBy  string
	ExpiresAt  time.Time
	AcceptedAt *time.Time
}

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

type UserResponse struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	AvatarURL string `json:"avatarUrl"`
	CreatedAt string `json:"createdAt"`
}

// CreateUserRequest is the body of the internal POST /internal/users
// route — called by Auth Service during signup, never by the gateway.
type CreateUserRequest struct {
	OrgID string `json:"orgId"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type LookupMatch struct {
	UserID string `json:"userId"`
	OrgID  string `json:"orgId"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type LookupResponse struct {
	Matches []LookupMatch `json:"matches"`
}

type UpdateMeRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

// ListUsersResponse is GET /orgs/{orgId}/users' body — the standard
// paginated list envelope from docs/architecture/api-spec.md
// §"Pagination, filtering, errors".
type ListUsersResponse struct {
	Data     []UserResponse `json:"data"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Total    int            `json:"total"`
}

// CreateInviteRequest is the body of POST /orgs/{orgId}/invites — an
// owner/admin inviting a new member at a chosen role (never "owner":
// ownership transfer isn't a Phase 2 flow). See RegisterRoutes' RequireRole
// gate and docs/architecture/api-spec.md §Users.
type CreateInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// InviteResponse is CreateInvite's response. DevToken is only ever
// non-empty when devExposeToken is set on CreateInviteUseCase — see its
// doc comment, which mirrors auth-service's RequestPasswordResetUseCase:
// there is no Notification Service yet (that's Phase 4) to email the
// invite link, so Phase 2 logs the raw token server-side and, in local
// dev only, returns it here so the accept flow is testable end-to-end
// without a real mailbox.
type InviteResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt string `json:"expiresAt"`
	DevToken  string `json:"devToken,omitempty"`
}

// InviteAcceptRequest is the body of the internal POST
// /internal/invites/accept route — called by Auth Service's AcceptInvite
// usecase, never directly by the gateway (see
// docs/architecture/api-spec.md, POST /invites/{token}/accept: Auth
// Service owns issuing tokens and creating the credentials row, so it's
// the one that fronts this public route and calls User Service
// internally, exactly like CreateUser during signup).
type InviteAcceptRequest struct {
	Token string `json:"token"`
	Name  string `json:"name"`
}

type InviteAcceptResponse struct {
	UserID string `json:"userId"`
	OrgID  string `json:"orgId"`
	Role   string `json:"role"`
	Email  string `json:"email"`
}

// UpdateRoleRequest is the body of PATCH /orgs/{orgId}/users/{userId}/role.
type UpdateRoleRequest struct {
	Role string `json:"role"`
}

// --- usecase/*.go: input for each use case's Execute. Not JSON wire
// structs (no json tags) — these are the usecase layer's own Go-to-Go
// call contract, passed by handler/ straight from a decoded request. ---

type CreateUserInput struct {
	OrgID string
	Email string
	Name  string
	Role  string
}

type UpdateProfileInput struct {
	OrgID     string
	UserID    string
	Name      string
	AvatarURL string
}

// ListUsersFilter is repository.Repository.List's pagination input.
type ListUsersFilter struct {
	Page     int
	PageSize int
}

type CreateInviteInput struct {
	OrgID     string
	InvitedBy string
	Email     string
	Role      string
}

type UpdateRoleInput struct {
	OrgID        string
	UserID       string
	Role         string
	CallerUserID string
}

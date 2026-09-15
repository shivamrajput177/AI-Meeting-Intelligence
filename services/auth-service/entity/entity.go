// Package entity holds Auth Service's plain data structs: its own
// internal model (Credentials, RefreshToken, PasswordResetToken,
// EmailMatch — what repository/client read and usecase operates on),
// the JSON wire shapes at this service's HTTP boundaries, and the plain
// input/output structs its usecase layer passes around internally.
// Nothing here has behavior (no methods, just fields, and json tags
// where the struct crosses the wire): it's data, not a class, which is
// what keeps it out of handler/, client/, and usecase/ — those packages
// hold the code that does something with an entity, this package only
// describes its shape.
package entity

import "time"

// RoleOwner mirrors usersvc/entity.RoleOwner. Duplicated, not imported —
// Auth Service never imports another service's Go packages, only calls
// its REST API (see client.OrgClient/UserClient) — that's what keeps
// services independently deployable in practice, not just on paper.
const RoleOwner = "owner"

// Credentials, RefreshToken, and PasswordResetToken are this service's
// own internal model — what repository reads out of Postgres and
// usecase operates on. None are JSON-tagged: they never cross the wire
// directly.
type Credentials struct {
	UserID         string
	OrgID          string
	PasswordHash   string
	Algo           string
	FailedAttempts int
	LockedUntil    *time.Time
	UpdatedAt      time.Time
}

type RefreshToken struct {
	ID         string
	UserID     string
	OrgID      string
	TokenHash  string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
}

type PasswordResetToken struct {
	ID        string
	UserID    string
	OrgID     string
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// EmailMatch mirrors usersvc/entity.EmailLookup — duplicated rather than
// imported so Auth Service has zero dependency on User Service's
// internals, matching every other service boundary in this repo. It's
// what client.UserClient.LookupByEmail returns, over the wire from User
// Service's own /internal/users/lookup route.
type EmailMatch struct {
	UserID string
	OrgID  string
	Role   string
	Status string
}

// --- handler/handler.go: this service's own REST API ---

type TokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

type SignupRequest struct {
	OrgName  string `json:"orgName"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	OrgID        string `json:"orgId"`
	RefreshToken string `json:"refreshToken"`
	Role         string `json:"role"`
}

type LogoutRequest struct {
	OrgID        string `json:"orgId"`
	RefreshToken string `json:"refreshToken"`
}

type ResetRequestRequest struct {
	Email string `json:"email"`
}

type ResetRequestResponse struct {
	// DevToken is only ever non-empty when AUTH_DEV_EXPOSE_RESET_TOKEN is
	// set — see usecase.RequestPasswordResetUseCase's doc comment. Never
	// set this env var in the public demo deployment.
	DevToken string `json:"devToken,omitempty"`
}

type ResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

// AcceptInviteRequest is the body of POST /invites/{token}/accept — the
// invite-flow counterpart to SignupRequest. The token itself travels in
// the path, not the body (see RegisterRoutes).
type AcceptInviteRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

// --- client/org_client.go, client/user_client.go: outbound calls to
// organization-service and user-service ---

type CreateOrgRequest struct {
	Name string `json:"name"`
}

type OrgResponse struct {
	ID string `json:"id"`
}

type CreateUserRequest struct {
	OrgID string `json:"orgId"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type UserResponse struct {
	ID string `json:"id"`
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

// InviteAcceptRequest/Response are the outbound call to User Service's
// internal POST /internal/invites/accept — mirrors
// usersvc/entity.InviteAcceptRequest/Response, duplicated rather than
// imported for the same reason CreateUserRequest is (see this file's own
// doc comment).
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

// --- usecase/*.go: input/output for each use case's Execute, and the
// TokenPair every "logs someone in" use case (signup, login, refresh)
// returns. Not JSON wire structs (no json tags) — these are the
// usecase layer's own Go-to-Go call contract, passed by handler/
// straight from a decoded request. ---

type SignupInput struct {
	OrgName  string
	Email    string
	Name     string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type RefreshInput struct {
	// OrgID is required because refresh tokens are looked up scoped to an
	// org (see repository.RefreshTokenRepository) — the client must have
	// kept it from its last login/signup response, alongside the refresh
	// token itself.
	OrgID        string
	RefreshToken string
	// Role is carried by the client from its last token pair. Phase 1
	// doesn't re-derive it from User Service on every refresh (that would
	// mean a synchronous call to another service on every refresh, for a
	// value that essentially never changes between two refreshes) — a
	// role change (Phase 2's PATCH .../role) takes effect on the user's
	// *next* login/refresh cycle once they present a stale role, which is
	// an acceptable staleness window, not silently ignored forever.
	Role string
}

type LogoutInput struct {
	OrgID        string
	RefreshToken string
}

type AcceptInviteInput struct {
	Token    string
	Name     string
	Password string
}

// TokenPair is what every route that "logs someone in" (signup, login,
// refresh) returns.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // seconds, for the client to know when to refresh
}

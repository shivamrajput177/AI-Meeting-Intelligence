// Package entity holds Auth Service's plain data structs — the JSON wire
// shapes at this service's HTTP boundaries, and the plain
// input/output structs its usecase layer passes around internally.
// Nothing here has behavior (no methods, just fields, and json tags
// where the struct crosses the wire): it's data, not a class, which is
// what keeps it out of handler/, client/, and usecase/ — those packages
// hold the code that does something with an entity, this package only
// describes its shape.
package entity

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
	// org (see domain.RefreshTokenRepository) — the client must have kept
	// it from its last login/signup response, alongside the refresh token
	// itself.
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

// TokenPair is what every route that "logs someone in" (signup, login,
// refresh) returns.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // seconds, for the client to know when to refresh
}

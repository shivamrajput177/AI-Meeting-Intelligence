// Package entity holds Auth Service's plain request/response structs —
// the JSON wire shapes at this service's boundaries. Nothing here has
// behavior (no methods, just fields and json tags): it's data, not a
// class, which is what keeps it out of handler/ and client/ — those
// packages hold the code that does something with an entity, this
// package only describes its shape. Split into the two boundaries that
// produce/consume it: this service's own REST API (handler/handler.go)
// and the outbound calls its client package makes to
// organization-service/user-service (client/org_client.go,
// client/user_client.go).
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

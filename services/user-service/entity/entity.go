// Package entity holds User Service's plain request/response structs —
// the JSON wire shapes at this service's REST boundary. Nothing here has
// behavior (no methods, just fields and json tags): it's data, not a
// class, which is what keeps it out of handler/ — that package holds the
// code that does something with an entity, this package only describes
// its shape. See handler/handler.go for where these are actually used.
package entity

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

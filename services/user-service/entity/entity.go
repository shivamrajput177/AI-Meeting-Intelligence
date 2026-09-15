// Package entity holds User Service's plain data structs — the JSON wire
// shapes at this service's REST boundary, and the plain input structs
// its usecase layer passes around internally. Nothing here has behavior
// (no methods, just fields, and json tags where the struct crosses the
// wire): it's data, not a class, which is what keeps it out of handler/
// and usecase/ — those packages hold the code that does something with
// an entity, this package only describes its shape.
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

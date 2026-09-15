// Package entity holds Organization Service's plain request/response
// structs — the JSON wire shapes at this service's REST boundary.
// Nothing here has behavior (no methods, just fields and json tags):
// it's data, not a class, which is what keeps it out of handler/ — that
// package holds the code that does something with an entity, this
// package only describes its shape. See handler/handler.go for where
// these are actually used.
package entity

// CreateOrgRequest is the body of both the internal POST /internal/orgs
// call (from Auth Service during signup) and, in a later phase, any
// public "create a second org" flow.
type CreateOrgRequest struct {
	Name string `json:"name"`
}

type OrgResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

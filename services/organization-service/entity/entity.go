// Package entity holds Organization Service's plain data structs: its
// own internal model (Organization — what repository reads out of
// Postgres and usecase operates on), the JSON wire shapes at this
// service's REST boundary, and the plain input struct its usecase layer
// passes around internally. Nothing here has behavior (no methods, just
// fields, and json tags where the struct crosses the wire): it's data,
// not a class, which is what keeps it out of handler/ and usecase/ —
// those packages hold the code that does something with an entity, this
// package only describes its shape.
package entity

import "time"

// Organization is this service's own internal model — what repository
// reads out of Postgres and usecase operates on. Not JSON-tagged: it
// never crosses the wire directly, handler.go always reshapes it into
// an OrgResponse first (see toOrgResponse).
type Organization struct {
	ID        string
	Name      string
	Slug      string
	Plan      string
	Status    string
	CreatedAt time.Time
}

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

// CreateOrgInput is usecase.CreateOrgUseCase.CreateOrg's input — the
// usecase layer's own Go-to-Go call contract, not a JSON wire struct
// (no json tags), passed by handler/ straight from a decoded
// CreateOrgRequest.
type CreateOrgInput struct {
	Name string
}

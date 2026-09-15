package domain

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

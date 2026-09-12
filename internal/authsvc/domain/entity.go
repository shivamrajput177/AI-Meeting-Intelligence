package domain

import "time"

// RoleOwner mirrors usersvc/domain.RoleOwner. Duplicated, not imported —
// Auth Service never imports another service's Go packages, only calls
// its REST API (see domain/ports.go's OrgClient/UserClient) — that's what
// keeps services independently deployable in practice, not just on paper.
const RoleOwner = "owner"

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

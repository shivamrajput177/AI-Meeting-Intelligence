package domain

import "context"

// Repository is what usecase depends on; repository/postgres implements
// it. Defining the interface here (not in the repository package) is what
// lets usecase be unit-tested against an in-memory fake with zero
// Postgres involved.
type Repository interface {
	Create(ctx context.Context, org *Organization) error
	GetByID(ctx context.Context, id string) (*Organization, error)
}

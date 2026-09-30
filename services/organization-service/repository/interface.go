// Package repository defines the interfaces usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements them. Keeping the interfaces here instead of off in some
// unrelated package is just where they belong — their one real
// implementation lives one directory down. usecase still only ever
// depends on these interface types, never on the concrete
// *postgres.OrgRepository/*postgres.IntegrationConfigRepository directly,
// which is what lets it be unit-tested against an in-memory fake with
// zero Postgres involved. Two interfaces, not one, since organizations
// and integration config are genuinely distinct data-access concerns —
// the same split several other services' own repository/interface.go
// files already use for their own distinct concerns.
package repository

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
)

type Repository interface {
	Create(ctx context.Context, org *entity.Organization) error
	GetByID(ctx context.Context, id string) (*entity.Organization, error)
}

// IntegrationConfigRepository backs Phase 4.5's PATCH
// /orgs/{orgId}/settings and Notification Service's own internal read of
// it — see entity.IntegrationConfig's own doc comment.
type IntegrationConfigRepository interface {
	GetIntegrationConfig(ctx context.Context, orgID string) (*entity.IntegrationConfig, error)
	UpdateIntegrationConfig(ctx context.Context, orgID string, input entity.UpdateIntegrationConfigInput) (*entity.IntegrationConfig, error)
}

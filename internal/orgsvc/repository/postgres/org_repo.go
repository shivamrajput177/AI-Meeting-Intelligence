// Package postgres implements orgsvc/domain.Repository against the org.*
// schema (see docs/architecture/database-schema.md). org.organizations is
// the tenant root table — it has no org_id column of its own to apply RLS
// against — so, unlike every other repository in this repo, there is no
// tenant-context transaction here.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/domain"
)

type OrgRepository struct {
	pool *pgxpool.Pool
}

func NewOrgRepository(pool *pgxpool.Pool) *OrgRepository {
	return &OrgRepository{pool: pool}
}

func (r *OrgRepository) Create(ctx context.Context, org *domain.Organization) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx,
		`INSERT INTO org.organizations (id, name, slug, plan, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		org.ID, org.Name, org.Slug, org.Plan, org.Status, org.CreatedAt,
	); err != nil {
		return err
	}

	// Every org gets a default quota row at creation — see
	// docs/architecture/database-schema.md's org.quotas table. Phase 1
	// doesn't enforce these limits yet (that's Organization Service's
	// GetQuotaUsage/enforcement work, deferred until there's real usage to
	// meter), but the row exists from day one so enforcement is additive
	// later, not a migration.
	if _, err := tx.Exec(ctx,
		`INSERT INTO org.quotas (org_id, max_users, max_minutes_per_month, retention_days)
		 VALUES ($1, 10, 1000, 365)`,
		org.ID,
	); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *OrgRepository) GetByID(ctx context.Context, id string) (*domain.Organization, error) {
	var org domain.Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, plan, status, created_at FROM org.organizations WHERE id = $1`,
		id,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Plan, &org.Status, &org.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrOrgNotFound
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

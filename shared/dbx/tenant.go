package dbx

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTenantTx opens a transaction, sets app.current_org for the duration
// of that transaction (which is what every tenant table's Row-Level
// Security policy filters on — see
// docs/architecture/database-schema.md §"Row-Level Security pattern"),
// runs fn, and commits or rolls back.
//
// Postgres does not allow SET LOCAL's value to be a bind parameter (SET is
// a utility statement, not part of the extended-query parameter grammar),
// so the org id is validated as a well-formed UUID and then interpolated
// directly — safe specifically because it's constrained to the UUID
// grammar first, not because string interpolation into SQL is fine in
// general.
func WithTenantTx(ctx context.Context, pool *pgxpool.Pool, orgID string, fn func(ctx context.Context, tx pgx.Tx) error) error {
	if _, err := uuid.Parse(orgID); err != nil {
		return fmt.Errorf("dbx: invalid org id %q: %w", orgID, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("dbx: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op if already committed

	if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.current_org = '%s'", orgID)); err != nil {
		return fmt.Errorf("dbx: set tenant context: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// WithBypassRLSTx opens a transaction with app.bypass_tenant_isolation set
// instead of app.current_org — the escape hatch a small number of
// legitimately cross-tenant internal queries need (today: exactly one —
// usersvc's LookupByEmail, see migrations/user/0001_init.up.sql for why).
// Every RLS policy this is meant to bypass must explicitly OR against
// this setting; it does nothing on tables whose policy doesn't check for
// it, by design — this is an opt-in per-table escape hatch, not a global
// RLS off-switch.
//
// Callers must use this only for the specific, narrow queries that
// actually need cross-tenant visibility, never as a general "make RLS
// stop complaining" shortcut.
func WithBypassRLSTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("dbx: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SET LOCAL app.bypass_tenant_isolation = 'on'"); err != nil {
		return fmt.Errorf("dbx: set bypass flag: %w", err)
	}

	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

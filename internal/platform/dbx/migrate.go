package dbx

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations applies every *.up.sql file under dir in filename order,
// tracked in a per-schema schema_migrations table so re-running on every
// service restart is a no-op. This is a deliberately small, dependency-free
// stand-in for golang-migrate: our migrations are plain DDL (no
// down-migration tooling, no dirty-state recovery), which is all a Phase 1
// walking skeleton needs — see docs/architecture/folder-structure.md for
// the per-service migrations/ layout this reads.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, schema string, migrations embed.FS, dir string) error {
	if _, err := pool.Exec(ctx, fmt.Sprintf(
		`CREATE SCHEMA IF NOT EXISTS %s; CREATE TABLE IF NOT EXISTS %s.schema_migrations (
			version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		quoteIdent(schema), quoteIdent(schema))); err != nil {
		return fmt.Errorf("dbx: ensure schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrations, dir)
	if err != nil {
		return fmt.Errorf("dbx: read migrations dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		var applied bool
		err := pool.QueryRow(ctx,
			fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s.schema_migrations WHERE version = $1)", quoteIdent(schema)),
			name,
		).Scan(&applied)
		if err != nil {
			return fmt.Errorf("dbx: check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		content, err := fs.ReadFile(migrations, path.Join(dir, name))
		if err != nil {
			return fmt.Errorf("dbx: read migration %s: %w", name, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("dbx: begin tx for %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, string(content)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("dbx: apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx,
			fmt.Sprintf("INSERT INTO %s.schema_migrations (version) VALUES ($1)", quoteIdent(schema)),
			name,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("dbx: record migration %s: %w", name, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("dbx: commit migration %s: %w", name, err)
		}
	}
	return nil
}

// quoteIdent is a minimal identifier quoter — fine here because schema
// names are hardcoded per-service constants (never user input), not a
// general-purpose SQL-injection defense.
func quoteIdent(ident string) string {
	return `"` + ident + `"`
}

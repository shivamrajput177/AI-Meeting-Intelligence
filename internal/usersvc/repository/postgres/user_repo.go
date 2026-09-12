// Package postgres implements usersvc/domain.Repository against the
// user.* schema (see docs/architecture/database-schema.md).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/dbx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/usersvc/domain"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	return dbx.WithTenantTx(ctx, r.pool, user.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO "user".users (id, org_id, email, name, role, status, avatar_url, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			user.ID, user.OrgID, user.Email, user.Name, user.Role, user.Status, user.AvatarURL,
			user.CreatedAt, user.UpdatedAt,
		)
		if isUniqueViolation(err) {
			return domain.ErrEmailInUse
		}
		return err
	})
}

func (r *UserRepository) GetByID(ctx context.Context, orgID, userID string) (*domain.User, error) {
	var user domain.User
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT id, org_id, email, name, role, status, COALESCE(avatar_url, ''), created_at, updated_at
			 FROM "user".users WHERE id = $1`,
			userID,
		).Scan(&user.ID, &user.OrgID, &user.Email, &user.Name, &user.Role, &user.Status,
			&user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, orgID, userID, name, avatarURL string) (*domain.User, error) {
	var user domain.User
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`UPDATE "user".users SET name = $1, avatar_url = $2, updated_at = now()
			 WHERE id = $3
			 RETURNING id, org_id, email, name, role, status, COALESCE(avatar_url, ''), created_at, updated_at`,
			name, avatarURL, userID,
		).Scan(&user.ID, &user.OrgID, &user.Email, &user.Name, &user.Role, &user.Status,
			&user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// LookupByEmail uses dbx.WithBypassRLSTx, not WithTenantTx — at login time
// the caller doesn't know their org_id yet (that's what this resolves),
// so there is no tenant to scope to; RLS's default fail-closed behavior
// (no app.current_org set => zero rows) would otherwise make this always
// return nothing. See migrations/user/0001_init.up.sql for the matching
// policy and WithBypassRLSTx's doc comment for why this is safe: it's an
// explicit, narrow, per-query opt-in, not a general RLS bypass. This
// method must never be reachable from a public route — only from
// /internal/users/lookup, itself guarded by RequireInternalToken.
func (r *UserRepository) LookupByEmail(ctx context.Context, email string) ([]domain.EmailLookup, error) {
	var results []domain.EmailLookup
	err := dbx.WithBypassRLSTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT id, org_id, role, status FROM "user".users WHERE email = $1 AND status = 'active'`,
			email,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var l domain.EmailLookup
			if err := rows.Scan(&l.UserID, &l.OrgID, &l.Role, &l.Status); err != nil {
				return err
			}
			results = append(results, l)
		}
		return rows.Err()
	})
	return results, err
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505) — used to turn a raw DB conflict into
// domain.ErrEmailInUse instead of leaking the constraint name.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

// Package postgres implements repository.Repository against the user.*
// schema (see docs/architecture/database-schema.md).
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/dbx"
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user *entity.User) error {
	return dbx.WithTenantTx(ctx, r.pool, user.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO "user".users (id, org_id, email, name, role, status, avatar_url, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			user.ID, user.OrgID, user.Email, user.Name, user.Role, user.Status, user.AvatarURL,
			user.CreatedAt, user.UpdatedAt,
		)
		if isUniqueViolation(err) {
			return apperr.Conflict("email already registered for this organization")
		}
		return err
	})
}

func (r *UserRepository) GetByID(ctx context.Context, orgID, userID string) (*entity.User, error) {
	var user entity.User
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT id, org_id, email, name, role, status, COALESCE(avatar_url, ''), created_at, updated_at
			 FROM "user".users WHERE id = $1`,
			userID,
		).Scan(&user.ID, &user.OrgID, &user.Email, &user.Name, &user.Role, &user.Status,
			&user.AvatarURL, &user.CreatedAt, &user.UpdatedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, orgID, userID, name, avatarURL string) (*entity.User, error) {
	var user entity.User
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
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// selectUserCols lists the columns List/GetByID/UpdateProfile/UpdateRole/
// Deactivate all scan in the same order, so a change to the row shape
// only needs updating in one place.
const selectUserCols = `id, org_id, email, name, role, status, COALESCE(avatar_url, ''), created_at, updated_at`

func scanUser(row pgx.Row) (*entity.User, error) {
	var u entity.User
	err := row.Scan(&u.ID, &u.OrgID, &u.Email, &u.Name, &u.Role, &u.Status, &u.AvatarURL, &u.CreatedAt, &u.UpdatedAt)
	return &u, err
}

func (r *UserRepository) List(ctx context.Context, orgID string, filter entity.ListUsersFilter) ([]*entity.User, int, error) {
	var items []*entity.User
	var total int
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM "user".users`).Scan(&total); err != nil {
			return err
		}

		offset := (filter.Page - 1) * filter.PageSize
		rows, err := tx.Query(ctx,
			`SELECT `+selectUserCols+` FROM "user".users ORDER BY created_at ASC LIMIT $1 OFFSET $2`,
			filter.PageSize, offset,
		)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			u, err := scanUser(rows)
			if err != nil {
				return err
			}
			items = append(items, u)
		}
		return rows.Err()
	})
	return items, total, err
}

func (r *UserRepository) UpdateRole(ctx context.Context, orgID, userID, role string) (*entity.User, error) {
	var user *entity.User
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`UPDATE "user".users SET role = $1, updated_at = now()
			 WHERE id = $2
			 RETURNING `+selectUserCols,
			role, userID,
		)
		u, err := scanUser(row)
		user = u
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Deactivate(ctx context.Context, orgID, userID string) (*entity.User, error) {
	var user *entity.User
	err := dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx,
			`UPDATE "user".users SET status = $1, updated_at = now()
			 WHERE id = $2
			 RETURNING `+selectUserCols,
			entity.StatusDeactivated, userID,
		)
		u, err := scanUser(row)
		user = u
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) CreateInvite(ctx context.Context, invite *entity.Invite) error {
	return dbx.WithTenantTx(ctx, r.pool, invite.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO "user".invites (id, org_id, email, role, token_hash, invited_by, expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			invite.ID, invite.OrgID, invite.Email, invite.Role, invite.TokenHash, invite.InvitedBy, invite.ExpiresAt,
		)
		return err
	})
}

// GetInviteByTokenHash uses dbx.WithBypassRLSTx, not WithTenantTx — see
// this method's doc comment on repository.Repository and
// migrations/0001_init.up.sql for why: the caller doesn't know the
// invite's org_id until this resolves it.
func (r *UserRepository) GetInviteByTokenHash(ctx context.Context, tokenHash string) (*entity.Invite, error) {
	var inv entity.Invite
	err := dbx.WithBypassRLSTx(ctx, r.pool, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT id, org_id, email, role, token_hash, invited_by, expires_at, accepted_at
			 FROM "user".invites WHERE token_hash = $1`,
			tokenHash,
		).Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.TokenHash, &inv.InvitedBy, &inv.ExpiresAt, &inv.AcceptedAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.BadRequest("invalid or expired invite token")
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

func (r *UserRepository) MarkInviteAccepted(ctx context.Context, orgID, inviteID string) error {
	return dbx.WithTenantTx(ctx, r.pool, orgID, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE "user".invites SET accepted_at = now() WHERE id = $1`, inviteID)
		return err
	})
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
func (r *UserRepository) LookupByEmail(ctx context.Context, email string) ([]entity.EmailLookup, error) {
	var results []entity.EmailLookup
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
			var l entity.EmailLookup
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
// violation (SQLSTATE 23505) — used to turn a raw DB conflict into a
// clean "email already registered" error instead of leaking the
// constraint name.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23505"
}

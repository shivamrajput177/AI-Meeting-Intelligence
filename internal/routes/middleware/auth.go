// Package middleware holds the API Gateway's own HTTP middleware —
// distinct from internal/platform/httpserver's shared middleware, which
// every service (gateway included) uses.
package middleware

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/jwtutil"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/redisx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

// Auth verifies the caller's JWT (signature + expiry, then a revocation
// check) and, on success, sets X-Org-Id/X-User-Id/X-Role headers on the
// *outbound* proxied request — this is the one place in the system that
// turns "a bearer token" into "a trusted identity" for every downstream
// service to read (see internal/platform/httpserver's contextFromHeaders,
// the receiving side of this).
//
// The Redis revocation check is treated as advisory, not authoritative:
// if Redis is unreachable, the request proceeds on the JWT's signature
// and expiry alone rather than failing the whole API. This is a
// deliberate availability-over-strictness call for Phase 1 — revocation
// is a defense-in-depth layer on top of short-lived (15m) access tokens,
// not the only thing standing between a stolen token and continued
// access — worth being able to defend as a trade-off, not an oversight.
func Auth(jwtSecret []byte, rdb *redis.Client, log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		token, ok := strings.CutPrefix(authHeader, "Bearer ")
		if !ok || token == "" {
			return apperr.Unauthorized("missing bearer token")
		}

		claims, err := jwtutil.ParseAccessToken(jwtSecret, token)
		if err != nil {
			return apperr.Unauthorized("invalid or expired token")
		}

		ctx, cancel := context.WithTimeout(c.Context(), 500*time.Millisecond)
		defer cancel()
		revoked, err := redisx.IsRevoked(ctx, rdb, claims.ID)
		if err != nil {
			log.Warn("revocation check failed, proceeding on JWT validity alone", slog.Any("err", err))
		} else if revoked {
			return apperr.Unauthorized("token has been revoked")
		}

		c.Request().Header.Set(reqctx.HeaderUserID, claims.Subject)
		c.Request().Header.Set(reqctx.HeaderOrgID, claims.OrgID)
		c.Request().Header.Set(reqctx.HeaderRole, claims.Role)
		return c.Next()
	}
}

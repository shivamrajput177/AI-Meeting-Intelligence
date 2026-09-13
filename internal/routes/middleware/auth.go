// Package middleware holds the API Gateway's own HTTP middleware —
// distinct from internal/platform/httpserver's shared middleware, which
// every service (gateway included) uses. Both are plain
// func(http.Handler) http.Handler, the standard net/http middleware shape
// — no framework needed.
package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/jwtutil"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/logger"
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
func Auth(jwtSecret []byte, rdb *redis.Client, log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(reqctx.HeaderRequestID)

			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || token == "" {
				apperr.Write(w, requestID, apperr.Unauthorized("missing bearer token"))
				return
			}

			claims, err := jwtutil.ParseAccessToken(jwtSecret, token)
			if err != nil {
				apperr.Write(w, requestID, apperr.Unauthorized("invalid or expired token"))
				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), 500*time.Millisecond)
			defer cancel()
			revoked, err := redisx.IsRevoked(ctx, rdb, claims.ID)
			if err != nil {
				log.Warn("revocation check failed, proceeding on JWT validity alone", "err", err)
			} else if revoked {
				apperr.Write(w, requestID, apperr.Unauthorized("token has been revoked"))
				return
			}

			r.Header.Set(reqctx.HeaderUserID, claims.Subject)
			r.Header.Set(reqctx.HeaderOrgID, claims.OrgID)
			r.Header.Set(reqctx.HeaderRole, claims.Role)
			next.ServeHTTP(w, r)
		})
	}
}

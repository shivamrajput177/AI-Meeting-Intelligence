package middleware

import (
	"net/http"
	"slices"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// RequireRole is the second of the two RBAC enforcement layers documented
// in docs/architecture/observability-security.md §2 ("gateway pre-check +
// per-service HTTP middleware re-check — the gateway is not trusted as
// the sole enforcement point").
//
// At the API Gateway it's chained right after Auth, which — for exactly
// this reason — overwrites reqctx's org/user/role in the request context
// with the JWT-verified values rather than leaving whatever
// ContextFromHeaders picked up from the original, untrusted caller. At
// every other service it reads the same reqctx.Role, populated there by
// ContextFromHeaders off the X-Role header the gateway already set and
// verified.
func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !slices.Contains(allowed, reqctx.Role(r.Context())) {
				apperr.Write(w, r.Header.Get(reqctx.HeaderRequestID), apperr.Forbidden("insufficient role for this action"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

package httpserver

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

// contextFromHeaders reads the org/user/role/request-id headers the API
// Gateway sets (after verifying the caller's JWT — see
// internal/routes/middleware) into a context.Context every handler in
// this service reads via r.Context() — with net/http there's only ever
// one request context, unlike Fiber's separate c.Context()/c.UserContext()
// distinction (a real bug caught during that framework's use: code was
// reading from the one that never had these values set).
//
// Trust model, stated plainly: Phase 1 has no NetworkPolicy or mTLS
// restricting who can call a service directly — a request that reaches
// this service with these headers already set is trusted at face value.
// That's fine for a local docker-compose network with no other tenants
// on it, and is exactly the gap docs/architecture/observability-security.md
// §2 names as a Phase 6 hardening item (a service mesh would enforce that
// only the gateway/other known services can reach this one at all).
func contextFromHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if v := r.Header.Get(reqctx.HeaderOrgID); v != "" {
			ctx = reqctx.WithOrgID(ctx, v)
		}
		if v := r.Header.Get(reqctx.HeaderUserID); v != "" {
			ctx = reqctx.WithUserID(ctx, v)
		}
		if v := r.Header.Get(reqctx.HeaderRole); v != "" {
			ctx = reqctx.WithRole(ctx, v)
		}
		if v := r.Header.Get(reqctx.HeaderRequestID); v != "" {
			ctx = reqctx.WithRequestID(ctx, v)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

package httpserver

import (
	"context"

	"github.com/gofiber/fiber/v2"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

// contextFromHeaders reads the org/user/role/request-id headers the API
// Gateway sets (after verifying the caller's JWT — see
// internal/routes/middleware) into a context.Context every handler in
// this service reads via c.UserContext(), not c.Context() (which is
// fasthttp's raw request context and never carries these values — using
// it directly for org/user/role was a bug caught before Phase 1's first
// build, not a real pattern to copy).
//
// Trust model, stated plainly: Phase 1 has no NetworkPolicy or mTLS
// restricting who can call a service directly — a request that reaches
// this service with these headers already set is trusted at face value.
// That's fine for a local docker-compose network with no other tenants
// on it, and is exactly the gap docs/architecture/observability-security.md
// §2 names as a Phase 6 hardening item (a service mesh would enforce that
// only the gateway/other known services can reach this one at all).
func contextFromHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var ctx context.Context = c.Context() // fasthttp.RequestCtx implements context.Context (deadline/cancellation)
		if v := c.Get(reqctx.HeaderOrgID); v != "" {
			ctx = reqctx.WithOrgID(ctx, v)
		}
		if v := c.Get(reqctx.HeaderUserID); v != "" {
			ctx = reqctx.WithUserID(ctx, v)
		}
		if v := c.Get(reqctx.HeaderRole); v != "" {
			ctx = reqctx.WithRole(ctx, v)
		}
		if v := c.Get(reqctx.HeaderRequestID); v != "" {
			ctx = reqctx.WithRequestID(ctx, v)
		}
		c.SetUserContext(ctx)
		return c.Next()
	}
}

package httpserver

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v2"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

// RequireInternalToken protects a service's /internal/* routes — the ones
// meant to be called only by other services, never by an external client
// through the gateway (e.g. Auth Service creating the org+user rows during
// signup, before any user JWT exists to authenticate the call).
//
// This is a deliberately simple shared-secret check for Phase 1. It's the
// same trust boundary a service mesh's mTLS would enforce more robustly —
// see docs/architecture/observability-security.md §2, which calls out
// mTLS via Linkerd as the Phase 6 hardening step this stands in for.
func RequireInternalToken(secret string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		got := c.Get(reqctx.HeaderInternalToken)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			return apperr.Unauthorized("missing or invalid internal service token")
		}
		return c.Next()
	}
}

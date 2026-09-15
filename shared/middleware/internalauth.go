package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// RequireInternalToken protects a service's /internal/* routes — the ones
// meant to be called only by other services, never by an external client
// through the gateway (e.g. Auth Service creating the org+user rows during
// signup, before any user JWT exists to authenticate the call). Wrap an
// individual route's handler with it, e.g.:
//
//	mux.Handle("POST /internal/orgs", middleware.RequireInternalToken(secret, httpserver.H(h.CreateOrg)))
//
// This is a deliberately simple shared-secret check for Phase 1. It's the
// same trust boundary a service mesh's mTLS would enforce more robustly —
// see docs/architecture/observability-security.md §2, which calls out
// mTLS via Linkerd as the Phase 6 hardening step this stands in for.
func RequireInternalToken(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get(reqctx.HeaderInternalToken)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			apperr.Write(w, r.Header.Get(reqctx.HeaderRequestID), apperr.Unauthorized("missing or invalid internal service token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

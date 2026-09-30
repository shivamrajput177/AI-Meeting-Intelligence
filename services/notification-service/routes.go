// routes.go mounts Notification Service's Handler methods onto a
// *http.ServeMux. GET /demo/board is deliberately not behind the
// gateway's JWT auth wrapper (see services/api-gateway/router.go's public
// section) — DemoBoard's own doc comment explains why a public mock board
// is the whole point. The other two routes are proxied behind the
// gateway's ordinary auth like everything else; this service re-checks
// the org match itself (requireSameOrg) the same defense-in-depth way
// every other {orgId}-scoped route in this repo does.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.Handle("GET /demo/board", httpserver.H(h.DemoBoard))
	mux.Handle("GET /orgs/{orgId}/mock-jira/board", httpserver.H(h.OrgMockJiraBoard))
	mux.Handle("PATCH /orgs/{orgId}/mock-jira/issues/{issueKey}", httpserver.H(h.TransitionMockIssue))
}

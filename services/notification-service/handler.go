// handler.go is Notification Service's REST API — new as of Phase 4.2
// (see main.go's own doc comment on why Phase 4.1 had none): the mock
// Jira board's own routes, per
// docs/architecture/deployment-demo-strategy.md §3's API additions table.
package main

import (
	"context"
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	getBoard        *usecase.GetMockBoardUseCase
	transition      *usecase.TransitionMockIssueUseCase
	testIntegration *usecase.TestIntegrationUseCase
	demoOrgID       string
}

func NewHandler(
	getBoard *usecase.GetMockBoardUseCase, transition *usecase.TransitionMockIssueUseCase,
	testIntegration *usecase.TestIntegrationUseCase, demoOrgID string,
) *Handler {
	return &Handler{getBoard: getBoard, transition: transition, testIntegration: testIntegration, demoOrgID: demoOrgID}
}

func toMockIssueResponse(it entity.MockJiraIssue) entity.MockJiraIssueResponse {
	return entity.MockJiraIssueResponse{IssueKey: it.IssueKey, ProjectKey: it.ProjectKey, Title: it.Title, Status: it.Status}
}

func (h *Handler) board(w http.ResponseWriter, r *http.Request, orgID string) error {
	issues, err := h.getBoard.GetBoard(r.Context(), orgID)
	if err != nil {
		return err
	}
	resp := entity.MockJiraBoardResponse{}
	for _, it := range issues {
		resp.Data = append(resp.Data, toMockIssueResponse(it))
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// DemoBoard serves GET /demo/board — public, no auth, scoped to a single
// operator-configured demo org (main.go's demoOrgID) rather than any org
// the caller names, since there's no JWT here to take an org id from.
// Unconfigured (empty demoOrgID) is a 404, not an empty board: an unset
// demo org means "this deployment never set one up," a materially
// different fact than "this org has no tickets yet."
func (h *Handler) DemoBoard(w http.ResponseWriter, r *http.Request) error {
	if h.demoOrgID == "" {
		return apperr.NotFound("demo org not configured")
	}
	return h.board(w, r, h.demoOrgID)
}

// OrgMockJiraBoard serves GET /orgs/{orgId}/mock-jira/board — the same
// board, for a real, authenticated org member.
func (h *Handler) OrgMockJiraBoard(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if err := requireSameOrg(r.Context(), orgID); err != nil {
		return err
	}
	return h.board(w, r, orgID)
}

// TransitionMockIssue serves PATCH
// /orgs/{orgId}/mock-jira/issues/{issueKey} — see
// TransitionMockIssueUseCase's doc comment for the bidirectional
// write-back this triggers.
func (h *Handler) TransitionMockIssue(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if err := requireSameOrg(r.Context(), orgID); err != nil {
		return err
	}
	var req entity.TransitionMockIssueRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.transition.TransitionMockIssue(r.Context(), orgID, r.PathValue("issueKey"), req.Status); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

// TestIntegration serves POST /orgs/{orgId}/integrations/test —
// owner/admin-only (see RegisterRoutes' requireOwnerOrAdmin gate, the
// per-service half of the defense-in-depth the gateway's own RequireRole
// is the other half of, same as organization-service's UpdateOrgSettings).
func (h *Handler) TestIntegration(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if err := requireSameOrg(r.Context(), orgID); err != nil {
		return err
	}
	var req entity.TestIntegrationRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.testIntegration.TestIntegration(r.Context(), orgID, req.Channel, req.To); err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, entity.TestIntegrationResponse{Channel: req.Channel, OK: true})
	return nil
}

// requireSameOrg is the same "a caller may only ever act on their own
// org" check every other service with an {orgId} path segment in this
// repo re-checks itself — the gateway's role/auth gate confirms *who* the
// caller is, never *which org* a forged path segment names.
func requireSameOrg(ctx context.Context, pathOrgID string) error {
	if callerOrg := reqctx.OrgID(ctx); callerOrg != "" && callerOrg != pathOrgID {
		return apperr.Forbidden("cannot access another organization")
	}
	return nil
}

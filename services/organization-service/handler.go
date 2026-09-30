// handler.go is Organization Service's own REST API. Every internal
// caller (Auth Service, during signup) and every gateway-forwarded
// request hit the same routes — there is no separate internal protocol,
// per docs/architecture/microservices.md §"Internal Communication".
package main

import (
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	createOrg               *usecase.CreateOrgUseCase
	getOrg                  *usecase.GetOrgUseCase
	updateIntegrationConfig *usecase.UpdateIntegrationConfigUseCase
	getIntegrationConfig    *usecase.GetIntegrationConfigUseCase
}

func NewHandler(
	createOrg *usecase.CreateOrgUseCase,
	getOrg *usecase.GetOrgUseCase,
	updateIntegrationConfig *usecase.UpdateIntegrationConfigUseCase,
	getIntegrationConfig *usecase.GetIntegrationConfigUseCase,
) *Handler {
	return &Handler{
		createOrg: createOrg, getOrg: getOrg,
		updateIntegrationConfig: updateIntegrationConfig, getIntegrationConfig: getIntegrationConfig,
	}
}

// fail writes the shared {"error": {...}} envelope via apperr.Write —
// every handler method below calls this at each of its error returns
// instead of writing one itself.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	apperr.Write(w, r.Header.Get(reqctx.HeaderRequestID), err)
}

// CreateOrg is called internally by Auth Service during signup (see
// docs/ROADMAP.md Phase 1 — there's no public "create a second org for an
// existing user" flow yet).
func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	var req entity.CreateOrgRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}

	org, err := h.createOrg.CreateOrg(r.Context(), entity.CreateOrgInput(req))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, toOrgResponse(org))
}

// GetOrg is reachable both via the gateway (a member viewing their own
// org) and directly from other services that need org metadata (e.g.
// Notification Service reading integration config, in a later phase).
func (h *Handler) GetOrg(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgId")

	// Gateway-forwarded requests carry the caller's org in context; a
	// member may only ever fetch their own org. Direct internal calls
	// (no reqctx org set) skip this check — they're already
	// token-authenticated by RequireInternalToken.
	if callerOrg := reqctx.OrgID(r.Context()); callerOrg != "" && callerOrg != orgID {
		fail(w, r, apperr.Forbidden("cannot access another organization"))
		return
	}

	org, err := h.getOrg.GetOrg(r.Context(), orgID)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toOrgResponse(org))
}

func toOrgResponse(o *entity.Organization) entity.OrgResponse {
	return entity.OrgResponse{
		ID:        o.ID,
		Name:      o.Name,
		Slug:      o.Slug,
		Plan:      o.Plan,
		Status:    o.Status,
		CreatedAt: o.CreatedAt.Format(time.RFC3339),
	}
}

func toIntegrationConfigResponse(c *entity.IntegrationConfig) entity.IntegrationConfigResponse {
	return entity.IntegrationConfigResponse{
		SlackWebhookURL: c.SlackWebhookURL, TicketProvider: c.TicketProvider,
		JiraBaseURL: c.JiraBaseURL, JiraProjectKey: c.JiraProjectKey,
		JiraAPITokenSecretRef: c.JiraAPITokenSecretRef,
	}
}

// UpdateOrgSettings serves PATCH /orgs/{orgId}/settings — owner/admin-only
// (see RegisterRoutes' requireOwnerOrAdmin gate, the per-service half of
// the defense-in-depth the gateway's own RequireRole is the other half
// of), and re-checks the caller's own org matches the path the same way
// GetOrg does, since role alone doesn't establish which org the caller
// belongs to.
func (h *Handler) UpdateOrgSettings(w http.ResponseWriter, r *http.Request) {
	orgID := r.PathValue("orgId")
	if callerOrg := reqctx.OrgID(r.Context()); callerOrg != "" && callerOrg != orgID {
		fail(w, r, apperr.Forbidden("cannot access another organization"))
		return
	}

	var req entity.UpdateIntegrationConfigRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	config, err := h.updateIntegrationConfig.UpdateIntegrationConfig(r.Context(), orgID, entity.UpdateIntegrationConfigInput(req))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toIntegrationConfigResponse(config))
}

// GetIntegrationConfigInternal serves
// GET /internal/orgs/{orgId}/integration-config — Notification Service's
// own read, at dispatch time (which Slack webhook URL and ticket provider
// an org is configured for). No org-match re-check here: the caller is
// already token-authenticated by RequireInternalToken, the same pattern
// every other /internal/* route in this repo uses.
func (h *Handler) GetIntegrationConfigInternal(w http.ResponseWriter, r *http.Request) {
	config, err := h.getIntegrationConfig.GetIntegrationConfig(r.Context(), r.PathValue("orgId"))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toIntegrationConfigResponse(config))
}

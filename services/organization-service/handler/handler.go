// Package handler is Organization Service's own REST API. Every internal
// caller (Auth Service, during signup) and every gateway-forwarded
// request hit the same routes — there is no separate internal protocol,
// per docs/architecture/microservices.md §"Internal Communication".
package handler

import (
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	createOrg *usecase.CreateOrgUseCase
	getOrg    *usecase.GetOrgUseCase
}

func NewHandler(createOrg *usecase.CreateOrgUseCase, getOrg *usecase.GetOrgUseCase) *Handler {
	return &Handler{createOrg: createOrg, getOrg: getOrg}
}

// CreateOrg is called internally by Auth Service during signup (see
// docs/ROADMAP.md Phase 1 — there's no public "create a second org for an
// existing user" flow yet).
func (h *Handler) CreateOrg(w http.ResponseWriter, r *http.Request) error {
	var req entity.CreateOrgRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}

	org, err := h.createOrg.Execute(r.Context(), entity.CreateOrgInput(req))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, toOrgResponse(org))
	return nil
}

// GetOrg is reachable both via the gateway (a member viewing their own
// org) and directly from other services that need org metadata (e.g.
// Notification Service reading integration config, in a later phase).
func (h *Handler) GetOrg(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")

	// Gateway-forwarded requests carry the caller's org in context; a
	// member may only ever fetch their own org. Direct internal calls
	// (no reqctx org set) skip this check — they're already
	// token-authenticated by RequireInternalToken.
	if callerOrg := reqctx.OrgID(r.Context()); callerOrg != "" && callerOrg != orgID {
		return apperr.Forbidden("cannot access another organization")
	}

	org, err := h.getOrg.Execute(r.Context(), orgID)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toOrgResponse(org))
	return nil
}

func toOrgResponse(o *domain.Organization) entity.OrgResponse {
	return entity.OrgResponse{
		ID:        o.ID,
		Name:      o.Name,
		Slug:      o.Slug,
		Plan:      o.Plan,
		Status:    o.Status,
		CreatedAt: o.CreatedAt.Format(time.RFC3339),
	}
}

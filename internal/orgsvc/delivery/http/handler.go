// Package http is Organization Service's own REST API. Every internal
// caller (Auth Service, during signup) and every gateway-forwarded
// request hit the same routes — there is no separate internal protocol,
// per docs/architecture/microservices.md §"Internal Communication".
package http

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/orgsvc/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

type Handler struct {
	createOrg *usecase.CreateOrgUseCase
	getOrg    *usecase.GetOrgUseCase
}

func NewHandler(createOrg *usecase.CreateOrgUseCase, getOrg *usecase.GetOrgUseCase) *Handler {
	return &Handler{createOrg: createOrg, getOrg: getOrg}
}

type createOrgRequest struct {
	Name string `json:"name"`
}

type orgResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// CreateOrg is called internally by Auth Service during signup (see
// docs/ROADMAP.md Phase 1 — there's no public "create a second org for an
// existing user" flow yet).
func (h *Handler) CreateOrg(c *fiber.Ctx) error {
	var req createOrgRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}

	org, err := h.createOrg.Execute(c.UserContext(), usecase.CreateOrgInput{Name: req.Name})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toOrgResponse(org))
}

// GetOrg is reachable both via the gateway (a member viewing their own
// org) and directly from other services that need org metadata (e.g.
// Notification Service reading integration config, in a later phase).
func (h *Handler) GetOrg(c *fiber.Ctx) error {
	orgID := c.Params("orgId")

	// Gateway-forwarded requests carry the caller's org in context; a
	// member may only ever fetch their own org. Direct internal calls
	// (no reqctx org set) skip this check — they're already
	// token-authenticated by RequireInternalToken.
	if callerOrg := reqctx.OrgID(c.UserContext()); callerOrg != "" && callerOrg != orgID {
		return apperr.Forbidden("cannot access another organization")
	}

	org, err := h.getOrg.Execute(c.UserContext(), orgID)
	if err != nil {
		return err
	}
	return c.JSON(toOrgResponse(org))
}

func toOrgResponse(o *domain.Organization) orgResponse {
	return orgResponse{
		ID:        o.ID,
		Name:      o.Name,
		Slug:      o.Slug,
		Plan:      o.Plan,
		Status:    o.Status,
		CreatedAt: o.CreatedAt.Format(time.RFC3339),
	}
}

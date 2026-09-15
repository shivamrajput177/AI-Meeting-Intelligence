// handler.go is User Service's own REST API — see
// docs/architecture/microservices.md §3 and docs/architecture/api-spec.md
// §Users (Phase 2's full RBAC work: invites, role changes, deactivation).
package main

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	createUser     *usecase.CreateUserUseCase
	getUser        *usecase.GetUserUseCase
	updateProfile  *usecase.UpdateProfileUseCase
	lookupByEmail  *usecase.LookupByEmailUseCase
	listUsers      *usecase.ListUsersUseCase
	createInvite   *usecase.CreateInviteUseCase
	acceptInvite   *usecase.AcceptInviteUseCase
	updateRole     *usecase.UpdateRoleUseCase
	deactivateUser *usecase.DeactivateUserUseCase
}

func NewHandler(
	createUser *usecase.CreateUserUseCase,
	getUser *usecase.GetUserUseCase,
	updateProfile *usecase.UpdateProfileUseCase,
	lookupByEmail *usecase.LookupByEmailUseCase,
	listUsers *usecase.ListUsersUseCase,
	createInvite *usecase.CreateInviteUseCase,
	acceptInvite *usecase.AcceptInviteUseCase,
	updateRole *usecase.UpdateRoleUseCase,
	deactivateUser *usecase.DeactivateUserUseCase,
) *Handler {
	return &Handler{
		createUser:     createUser,
		getUser:        getUser,
		updateProfile:  updateProfile,
		lookupByEmail:  lookupByEmail,
		listUsers:      listUsers,
		createInvite:   createInvite,
		acceptInvite:   acceptInvite,
		updateRole:     updateRole,
		deactivateUser: deactivateUser,
	}
}

func toUserResponse(u *entity.User) entity.UserResponse {
	return entity.UserResponse{
		ID: u.ID, OrgID: u.OrgID, Email: u.Email, Name: u.Name,
		Role: u.Role, Status: u.Status, AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

// --- internal routes (called by Auth Service, not the gateway) ---

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) error {
	var req entity.CreateUserRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	user, err := h.createUser.CreateUser(r.Context(), entity.CreateUserInput(req))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, toUserResponse(user))
	return nil
}

func (h *Handler) LookupByEmail(w http.ResponseWriter, r *http.Request) error {
	email := r.URL.Query().Get("email")
	if email == "" {
		return apperr.BadRequest("email query parameter is required")
	}
	matches, err := h.lookupByEmail.LookupByEmail(r.Context(), email)
	if err != nil {
		return err
	}
	resp := entity.LookupResponse{}
	for _, m := range matches {
		resp.Matches = append(resp.Matches, entity.LookupMatch(m))
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// AcceptInvite is called by Auth Service's own AcceptInvite usecase — see
// entity.InviteAcceptRequest's doc comment for why Auth Service, not the
// gateway, fronts the public POST /invites/{token}/accept route.
func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) error {
	var req entity.InviteAcceptRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	user, err := h.acceptInvite.AcceptInvite(r.Context(), req.Token, req.Name)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, entity.InviteAcceptResponse{
		UserID: user.ID, OrgID: user.OrgID, Role: user.Role, Email: user.Email,
	})
	return nil
}

// --- routes reachable via the gateway ---

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) error {
	orgID, userID := reqctx.OrgID(r.Context()), reqctx.UserID(r.Context())
	user, err := h.getUser.GetUser(r.Context(), orgID, userID)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	var req entity.UpdateMeRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	orgID, userID := reqctx.OrgID(r.Context()), reqctx.UserID(r.Context())
	user, err := h.updateProfile.UpdateProfile(r.Context(), entity.UpdateProfileInput{
		OrgID: orgID, UserID: userID, Name: req.Name, AvatarURL: req.AvatarURL,
	})
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

// GetUser lets another service (or the gateway, for a member viewing a
// teammate) fetch one user by id — see
// docs/architecture/microservices.md §3 ("used internally too").
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if callerOrg := reqctx.OrgID(r.Context()); callerOrg != "" && callerOrg != orgID {
		return apperr.Forbidden("cannot access another organization's users")
	}
	user, err := h.getUser.GetUser(r.Context(), orgID, r.PathValue("userId"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

// requireSameOrg is the same "a caller may only ever act on their own
// org" check GetUser already does above, factored out since every RBAC
// route below needs it too: the gateway's RequireRole gate confirms
// *what role* the caller has, never *which org* the {orgId} path segment
// names, so a forged path could otherwise reach across tenants.
func requireSameOrg(ctx context.Context, pathOrgID string) error {
	if callerOrg := reqctx.OrgID(ctx); callerOrg != "" && callerOrg != pathOrgID {
		return apperr.Forbidden("cannot access another organization")
	}
	return nil
}

// ListUsers is reachable by any org member (see RegisterRoutes — no
// RequireRole gate on this one, unlike the routes below it).
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if err := requireSameOrg(r.Context(), orgID); err != nil {
		return err
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 20
	}

	items, total, err := h.listUsers.ListUsers(r.Context(), orgID, entity.ListUsersFilter{Page: page, PageSize: pageSize})
	if err != nil {
		return err
	}
	resp := entity.ListUsersResponse{Page: page, PageSize: pageSize, Total: total}
	for _, u := range items {
		resp.Data = append(resp.Data, toUserResponse(u))
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// CreateInvite is owner/admin-only — see RegisterRoutes' RequireRole gate,
// the per-service half of the defense-in-depth the gateway's own
// RequireRole is the other half of (docs/architecture/observability-security.md §2).
func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if err := requireSameOrg(r.Context(), orgID); err != nil {
		return err
	}

	var req entity.CreateInviteRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	invite, devToken, err := h.createInvite.CreateInvite(r.Context(), entity.CreateInviteInput{
		OrgID: orgID, InvitedBy: reqctx.UserID(r.Context()), Email: req.Email, Role: req.Role,
	})
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, entity.InviteResponse{
		ID: invite.ID, Email: invite.Email, Role: invite.Role,
		ExpiresAt: invite.ExpiresAt.Format(time.RFC3339), DevToken: devToken,
	})
	return nil
}

// UpdateRole is owner/admin-only — see RegisterRoutes' RequireRole gate.
func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if err := requireSameOrg(r.Context(), orgID); err != nil {
		return err
	}

	var req entity.UpdateRoleRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	user, err := h.updateRole.UpdateRole(r.Context(), entity.UpdateRoleInput{
		OrgID: orgID, UserID: r.PathValue("userId"), Role: req.Role, CallerUserID: reqctx.UserID(r.Context()),
	})
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

// DeactivateUser is owner/admin-only — see RegisterRoutes' RequireRole gate.
func (h *Handler) DeactivateUser(w http.ResponseWriter, r *http.Request) error {
	orgID := r.PathValue("orgId")
	if err := requireSameOrg(r.Context(), orgID); err != nil {
		return err
	}

	if _, err := h.deactivateUser.DeactivateUser(r.Context(), orgID, r.PathValue("userId"), reqctx.UserID(r.Context())); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

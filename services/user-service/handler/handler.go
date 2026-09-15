// Package handler is User Service's own REST API — see
// docs/architecture/microservices.md §3 and
// docs/architecture/api-spec.md §Users (only the first two rows of that
// table are wired in Phase 1; the rest is Phase 2's RBAC work).
package handler

import (
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/user-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	createUser    *usecase.CreateUserUseCase
	getUser       *usecase.GetUserUseCase
	updateProfile *usecase.UpdateProfileUseCase
	lookupByEmail *usecase.LookupByEmailUseCase
}

func NewHandler(
	createUser *usecase.CreateUserUseCase,
	getUser *usecase.GetUserUseCase,
	updateProfile *usecase.UpdateProfileUseCase,
	lookupByEmail *usecase.LookupByEmailUseCase,
) *Handler {
	return &Handler{
		createUser:    createUser,
		getUser:       getUser,
		updateProfile: updateProfile,
		lookupByEmail: lookupByEmail,
	}
}

func toUserResponse(u *domain.User) entity.UserResponse {
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
		resp.Matches = append(resp.Matches, entity.LookupMatch{UserID: m.UserID, OrgID: m.OrgID, Role: m.Role, Status: m.Status})
	}
	httpserver.JSON(w, http.StatusOK, resp)
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

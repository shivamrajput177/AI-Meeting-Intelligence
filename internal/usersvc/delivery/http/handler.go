// Package http is User Service's own REST API — see
// docs/architecture/microservices.md §3 and
// docs/architecture/api-spec.md §Users (only the first two rows of that
// table are wired in Phase 1; the rest is Phase 2's RBAC work).
package http

import (
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/usersvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/usersvc/usecase"
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

type userResponse struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	AvatarURL string `json:"avatarUrl"`
	CreatedAt string `json:"createdAt"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
		ID: u.ID, OrgID: u.OrgID, Email: u.Email, Name: u.Name,
		Role: u.Role, Status: u.Status, AvatarURL: u.AvatarURL,
		CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

// --- internal routes (called by Auth Service, not the gateway) ---

type createUserRequest struct {
	OrgID string `json:"orgId"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) error {
	var req createUserRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	user, err := h.createUser.Execute(r.Context(), usecase.CreateUserInput{
		OrgID: req.OrgID, Email: req.Email, Name: req.Name, Role: req.Role,
	})
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, toUserResponse(user))
	return nil
}

type lookupResponse struct {
	Matches []struct {
		UserID string `json:"userId"`
		OrgID  string `json:"orgId"`
		Role   string `json:"role"`
		Status string `json:"status"`
	} `json:"matches"`
}

func (h *Handler) LookupByEmail(w http.ResponseWriter, r *http.Request) error {
	email := r.URL.Query().Get("email")
	if email == "" {
		return apperr.BadRequest("email query parameter is required")
	}
	matches, err := h.lookupByEmail.Execute(r.Context(), email)
	if err != nil {
		return err
	}
	resp := lookupResponse{}
	for _, m := range matches {
		resp.Matches = append(resp.Matches, struct {
			UserID string `json:"userId"`
			OrgID  string `json:"orgId"`
			Role   string `json:"role"`
			Status string `json:"status"`
		}{UserID: m.UserID, OrgID: m.OrgID, Role: m.Role, Status: m.Status})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// --- routes reachable via the gateway ---

func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) error {
	orgID, userID := reqctx.OrgID(r.Context()), reqctx.UserID(r.Context())
	user, err := h.getUser.Execute(r.Context(), orgID, userID)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

type updateMeRequest struct {
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	var req updateMeRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	orgID, userID := reqctx.OrgID(r.Context()), reqctx.UserID(r.Context())
	user, err := h.updateProfile.Execute(r.Context(), usecase.UpdateProfileInput{
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
	user, err := h.getUser.Execute(r.Context(), orgID, r.PathValue("userId"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toUserResponse(user))
	return nil
}

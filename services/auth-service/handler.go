// handler.go is Auth Service's REST API — see
// docs/architecture/microservices.md §2 and docs/architecture/api-spec.md
// §Auth.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
)

type Handler struct {
	signup       *usecase.SignupUseCase
	login        *usecase.LoginUseCase
	refresh      *usecase.RefreshUseCase
	logout       *usecase.LogoutUseCase
	requestReset *usecase.RequestPasswordResetUseCase
	confirmReset *usecase.ConfirmPasswordResetUseCase
	acceptInvite *usecase.AcceptInviteUseCase
}

func NewHandler(
	signup *usecase.SignupUseCase,
	login *usecase.LoginUseCase,
	refresh *usecase.RefreshUseCase,
	logout *usecase.LogoutUseCase,
	requestReset *usecase.RequestPasswordResetUseCase,
	confirmReset *usecase.ConfirmPasswordResetUseCase,
	acceptInvite *usecase.AcceptInviteUseCase,
) *Handler {
	return &Handler{
		signup: signup, login: login, refresh: refresh, logout: logout,
		requestReset: requestReset, confirmReset: confirmReset,
		acceptInvite: acceptInvite,
	}
}

func toTokenResponse(t *entity.TokenPair) entity.TokenResponse {
	return entity.TokenResponse{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresIn: t.ExpiresIn}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) error {
	var req entity.SignupRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	tokens, err := h.signup.Signup(r.Context(), entity.SignupInput(req))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, toTokenResponse(tokens))
	return nil
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) error {
	var req entity.LoginRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	tokens, err := h.login.Login(r.Context(), entity.LoginInput(req))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toTokenResponse(tokens))
	return nil
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) error {
	var req entity.RefreshRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	tokens, err := h.refresh.Refresh(r.Context(), entity.RefreshInput(req))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toTokenResponse(tokens))
	return nil
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	var req entity.LogoutRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.logout.Logout(r.Context(), entity.LogoutInput(req)); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) error {
	var req entity.ResetRequestRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	devToken, err := h.requestReset.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, entity.ResetRequestResponse{DevToken: devToken})
	return nil
}

func (h *Handler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) error {
	var req entity.ResetConfirmRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.confirmReset.ConfirmPasswordReset(r.Context(), req.Token, req.NewPassword); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

// AcceptInvite is the invite-flow counterpart to Signup — see
// entity.AcceptInviteInput's doc comment. The token travels in the path
// (POST /invites/{token}/accept), not the body.
func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) error {
	var req entity.AcceptInviteRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	tokens, err := h.acceptInvite.AcceptInvite(r.Context(), entity.AcceptInviteInput{
		Token: r.PathValue("token"), Name: req.Name, Password: req.Password,
	})
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, toTokenResponse(tokens))
	return nil
}

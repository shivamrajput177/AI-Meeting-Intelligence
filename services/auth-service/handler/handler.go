// Package handler is Auth Service's REST API — see
// docs/architecture/microservices.md §2 and docs/architecture/api-spec.md
// §Auth.
package handler

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
}

func NewHandler(
	signup *usecase.SignupUseCase,
	login *usecase.LoginUseCase,
	refresh *usecase.RefreshUseCase,
	logout *usecase.LogoutUseCase,
	requestReset *usecase.RequestPasswordResetUseCase,
	confirmReset *usecase.ConfirmPasswordResetUseCase,
) *Handler {
	return &Handler{
		signup: signup, login: login, refresh: refresh, logout: logout,
		requestReset: requestReset, confirmReset: confirmReset,
	}
}

func toTokenResponse(t *usecase.TokenPair) entity.TokenResponse {
	return entity.TokenResponse{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresIn: t.ExpiresIn}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) error {
	var req entity.SignupRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	tokens, err := h.signup.Execute(r.Context(), usecase.SignupInput{
		OrgName: req.OrgName, Email: req.Email, Name: req.Name, Password: req.Password,
	})
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
	tokens, err := h.login.Execute(r.Context(), usecase.LoginInput{Email: req.Email, Password: req.Password})
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
	tokens, err := h.refresh.Execute(r.Context(), usecase.RefreshInput{
		OrgID: req.OrgID, RefreshToken: req.RefreshToken, Role: req.Role,
	})
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
	if err := h.logout.Execute(r.Context(), usecase.LogoutInput{OrgID: req.OrgID, RefreshToken: req.RefreshToken}); err != nil {
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
	devToken, err := h.requestReset.Execute(r.Context(), req.Email)
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
	if err := h.confirmReset.Execute(r.Context(), req.Token, req.NewPassword); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

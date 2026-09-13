// Package http is Auth Service's REST API — see
// docs/architecture/microservices.md §2 and docs/architecture/api-spec.md
// §Auth.
package http

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpserver"
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

type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
}

func toTokenResponse(t *usecase.TokenPair) tokenResponse {
	return tokenResponse{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresIn: t.ExpiresIn}
}

type signupRequest struct {
	OrgName  string `json:"orgName"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) error {
	var req signupRequest
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

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
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

type refreshRequest struct {
	OrgID        string `json:"orgId"`
	RefreshToken string `json:"refreshToken"`
	Role         string `json:"role"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) error {
	var req refreshRequest
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

type logoutRequest struct {
	OrgID        string `json:"orgId"`
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) error {
	var req logoutRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.logout.Execute(r.Context(), usecase.LogoutInput{OrgID: req.OrgID, RefreshToken: req.RefreshToken}); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

type resetRequestRequest struct {
	Email string `json:"email"`
}

type resetRequestResponse struct {
	// DevToken is only ever non-empty when AUTH_DEV_EXPOSE_RESET_TOKEN is
	// set — see usecase.RequestPasswordResetUseCase's doc comment. Never
	// set this env var in the public demo deployment.
	DevToken string `json:"devToken,omitempty"`
}

func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) error {
	var req resetRequestRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	devToken, err := h.requestReset.Execute(r.Context(), req.Email)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, resetRequestResponse{DevToken: devToken})
	return nil
}

type resetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func (h *Handler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) error {
	var req resetConfirmRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.confirmReset.Execute(r.Context(), req.Token, req.NewPassword); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

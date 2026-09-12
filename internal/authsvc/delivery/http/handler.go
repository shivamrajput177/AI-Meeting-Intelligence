// Package http is Auth Service's REST API — see
// docs/architecture/microservices.md §2 and docs/architecture/api-spec.md
// §Auth.
package http

import (
	"github.com/gofiber/fiber/v2"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
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

func (h *Handler) Signup(c *fiber.Ctx) error {
	var req signupRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	tokens, err := h.signup.Execute(c.Context(), usecase.SignupInput{
		OrgName: req.OrgName, Email: req.Email, Name: req.Name, Password: req.Password,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(toTokenResponse(tokens))
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	tokens, err := h.login.Execute(c.Context(), usecase.LoginInput{Email: req.Email, Password: req.Password})
	if err != nil {
		return err
	}
	return c.JSON(toTokenResponse(tokens))
}

type refreshRequest struct {
	OrgID        string `json:"orgId"`
	RefreshToken string `json:"refreshToken"`
	Role         string `json:"role"`
}

func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req refreshRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	tokens, err := h.refresh.Execute(c.Context(), usecase.RefreshInput{
		OrgID: req.OrgID, RefreshToken: req.RefreshToken, Role: req.Role,
	})
	if err != nil {
		return err
	}
	return c.JSON(toTokenResponse(tokens))
}

type logoutRequest struct {
	OrgID        string `json:"orgId"`
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) Logout(c *fiber.Ctx) error {
	var req logoutRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.logout.Execute(c.Context(), usecase.LogoutInput{OrgID: req.OrgID, RefreshToken: req.RefreshToken}); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
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

func (h *Handler) RequestPasswordReset(c *fiber.Ctx) error {
	var req resetRequestRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	devToken, err := h.requestReset.Execute(c.Context(), req.Email)
	if err != nil {
		return err
	}
	return c.JSON(resetRequestResponse{DevToken: devToken})
}

type resetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func (h *Handler) ConfirmPasswordReset(c *fiber.Ctx) error {
	var req resetConfirmRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if err := h.confirmReset.Execute(c.Context(), req.Token, req.NewPassword); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

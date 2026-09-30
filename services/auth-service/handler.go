// handler.go is Auth Service's REST API — see
// docs/architecture/microservices.md §2 and docs/architecture/api-spec.md
// §Auth.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
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

// fail writes the shared {"error": {...}} envelope via apperr.Write —
// every handler method below calls this at each of its error returns
// instead of writing one itself.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	apperr.Write(w, r.Header.Get(reqctx.HeaderRequestID), err)
}

func toTokenResponse(t *entity.TokenPair) entity.TokenResponse {
	return entity.TokenResponse{AccessToken: t.AccessToken, RefreshToken: t.RefreshToken, ExpiresIn: t.ExpiresIn}
}

func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	var req entity.SignupRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	tokens, err := h.signup.Signup(r.Context(), entity.SignupInput(req))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, toTokenResponse(tokens))
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req entity.LoginRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	tokens, err := h.login.Login(r.Context(), entity.LoginInput(req))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toTokenResponse(tokens))
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req entity.RefreshRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	tokens, err := h.refresh.Refresh(r.Context(), entity.RefreshInput(req))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toTokenResponse(tokens))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req entity.LogoutRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	if err := h.logout.Logout(r.Context(), entity.LogoutInput(req)); err != nil {
		fail(w, r, err)
		return
	}
	httpserver.NoContent(w)
}

func (h *Handler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req entity.ResetRequestRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	devToken, err := h.requestReset.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, entity.ResetRequestResponse{DevToken: devToken})
}

func (h *Handler) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req entity.ResetConfirmRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	if err := h.confirmReset.ConfirmPasswordReset(r.Context(), req.Token, req.NewPassword); err != nil {
		fail(w, r, err)
		return
	}
	httpserver.NoContent(w)
}

// AcceptInvite is the invite-flow counterpart to Signup — see
// entity.AcceptInviteInput's doc comment. The token travels in the path
// (POST /invites/{token}/accept), not the body.
func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req entity.AcceptInviteRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	tokens, err := h.acceptInvite.AcceptInvite(r.Context(), entity.AcceptInviteInput{
		Token: r.PathValue("token"), Name: req.Name, Password: req.Password,
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, toTokenResponse(tokens))
}

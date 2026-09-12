package domain

import "github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"

var (
	ErrInvalidCredentials = apperr.Unauthorized("invalid email or password")
	ErrAccountLocked      = apperr.Unauthorized("account temporarily locked, try again later")
	ErrInvalidRefresh     = apperr.Unauthorized("invalid or expired refresh token")
	ErrInvalidResetToken  = apperr.BadRequest("invalid or expired reset token")
)

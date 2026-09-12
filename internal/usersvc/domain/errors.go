package domain

import "github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"

var (
	ErrUserNotFound = apperr.NotFound("user not found")
	ErrEmailInUse   = apperr.Conflict("email already registered for this organization")
	ErrInvalidLogin = apperr.Unauthorized("invalid email or password")
)

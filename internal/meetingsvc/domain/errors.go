package domain

import "github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"

var (
	ErrMeetingNotFound = apperr.NotFound("meeting not found")
	ErrInvalidStatus   = apperr.BadRequest("invalid status")
)

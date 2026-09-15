package domain

import "github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"

var (
	ErrMeetingNotFound = apperr.NotFound("meeting not found")
	ErrInvalidStatus   = apperr.BadRequest("invalid status")
)

package domain

import "github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"

var (
	ErrOrgNotFound = apperr.NotFound("organization not found")
)

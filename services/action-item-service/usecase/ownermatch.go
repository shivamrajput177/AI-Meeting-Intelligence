package usecase

import (
	"strings"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

// matchOwner is the "best-guess owner" half of
// docs/architecture/microservices.md §8 — a pure function (no I/O, easy
// to unit test) matching an LLM-extracted raw name string against a
// meeting's participant list. Matching is deliberately simple for a
// walking-skeleton pass: exact, case-insensitive comparison against each
// participant's display name, then against the local part of their email
// (the "shivam" in "shivam@example.com") as a fallback for a first-name
// mention a display name doesn't cover. No fuzzy/partial matching — a
// wrong owner guess is worse than no guess, and the raw name string is
// always preserved either way (see entity.ActionItem.OwnerRawName) so a
// human can correct a miss via PATCH /action-items/{id}.
func matchOwner(rawName string, participants []entity.Participant) *string {
	rawName = strings.TrimSpace(rawName)
	if rawName == "" {
		return nil
	}
	lower := strings.ToLower(rawName)

	for _, p := range participants {
		if strings.ToLower(strings.TrimSpace(p.DisplayName)) == lower && p.UserID != nil {
			return p.UserID
		}
	}
	for _, p := range participants {
		local, _, found := strings.Cut(p.Email, "@")
		if found && strings.ToLower(local) == lower && p.UserID != nil {
			return p.UserID
		}
	}
	return nil
}

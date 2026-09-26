// Package participants defines the Client interface usecase depends on;
// participants/http, right below this package in the same tree,
// implements it against Meeting Service's internal REST API — see that
// package's doc comment. Keeping the interface here instead of off in
// some unrelated package is just where it belongs — its one real
// implementation lives one directory down.
package participants

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

type Client interface {
	ListParticipants(ctx context.Context, orgID, meetingID string) ([]entity.Participant, error)
}

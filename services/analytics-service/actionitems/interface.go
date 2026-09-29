// Package actionitems defines the Client interface usecase depends on;
// actionitems/http, right below this package in the same tree, implements
// it against Action Item Service's internal REST API. Keeping the
// interface here instead of off in some unrelated package is just where
// it belongs — its one real implementation lives one directory down.
package actionitems

import "context"

// Item is this service's own Go-to-Go shape for one action item, as
// needed for the opened-rollup: just enough to attribute an "opened"
// count to an owner, nothing else.
type Item struct {
	ID          string
	OwnerUserID *string
}

type Client interface {
	// ListForMeeting returns every action item action-item-service has on
	// record for meetingID — this service's own read for
	// action-item.extracted.v1, which only carries a batch count (see
	// entity.ActionItemExtractedEvent), not the individual items'
	// owner_user_id the per-owner opened-rollup needs.
	ListForMeeting(ctx context.Context, orgID, meetingID string) ([]Item, error)
}

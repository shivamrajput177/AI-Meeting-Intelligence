// Package meetings defines the Client interface usecase depends on;
// meetings/http, right below this package in the same tree, implements it
// against Meeting Service's internal REST API. Keeping the interface here
// instead of off in some unrelated package is just where it belongs — its
// one real implementation lives one directory down.
package meetings

import "context"

// Meeting is this service's own Go-to-Go shape for the fields a
// notification message needs — not JSON-tagged.
type Meeting struct {
	Title     string
	CreatedBy string
}

type Client interface {
	GetMeeting(ctx context.Context, orgID, meetingID string) (*Meeting, error)
}

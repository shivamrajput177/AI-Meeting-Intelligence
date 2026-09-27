// Package meetings defines the Client interface usecase depends on;
// meetings/http, right below this package in the same tree, implements
// it against Meeting Service's internal REST API — see that package's doc
// comment. Keeping the interface here instead of off in some unrelated
// package is just where it belongs — its one real implementation lives
// one directory down.
package meetings

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
)

type Client interface {
	GetMeeting(ctx context.Context, orgID, meetingID string) (*entity.Meeting, error)

	// ListMeetings feeds ReindexUseCase's full-org backfill — see that
	// usecase's doc comment for the dev-scale, single-page limitation
	// this carries (Meeting Service's own ListMeetingsUseCase caps
	// pageSize at 100 regardless of what's asked for).
	ListMeetings(ctx context.Context, orgID string, page, pageSize int) (meetings []entity.Meeting, total int, err error)
}

// Package meetings defines the Client interface usecase depends on;
// meetings/http, right below this package in the same tree, implements it
// against Meeting Service's internal REST API. Keeping the interface here
// instead of off in some unrelated package is just where it belongs — its
// one real implementation lives one directory down.
package meetings

import "context"

type Client interface {
	// GetDurationMinutes looks up meetingID's recorded duration, rounding
	// seconds down to whole minutes. A meeting with no recorded duration
	// (DurationSeconds nil — see meetingsvc/entity.MeetingResponse) returns
	// 0, not an error: analytics.meeting_daily_rollup's total_minutes is
	// just uncounted for that meeting, the same trade-off as any other
	// best-effort read model in this service.
	GetDurationMinutes(ctx context.Context, orgID, meetingID string) (int, error)
}

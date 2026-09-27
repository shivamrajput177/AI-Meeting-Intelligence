package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/meetings"
)

// resolveMeetingTitles looks up each distinct meeting id in meetingIDs
// exactly once and returns a map for the caller to look titles up from —
// shared by SearchUseCase, SimilarMeetingsUseCase, and AskUseCase, all of
// which turn a list of chunk-level hits (possibly several per meeting)
// into a response that needs each hit's meeting title. A title lookup
// that 404s (e.g. the meeting was deleted after being embedded, before
// meeting.deleted.v1 cascade cleanup — Phase 3 doesn't implement that
// cleanup yet) falls back to the id itself rather than failing the whole
// request over one stale reference.
func resolveMeetingTitles(ctx context.Context, client meetings.Client, orgID string, meetingIDs []string) map[string]string {
	titles := make(map[string]string, len(meetingIDs))
	seen := make(map[string]bool, len(meetingIDs))
	for _, id := range meetingIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		m, err := client.GetMeeting(ctx, orgID, id)
		if err != nil {
			titles[id] = id
			continue
		}
		titles[id] = m.Title
	}
	return titles
}

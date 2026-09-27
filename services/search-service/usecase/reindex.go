package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// reindexPageSize matches Meeting Service's own ListMeetingsUseCase page
// size cap (100) — see meetings.Client.ListMeetings's doc comment for the
// dev-scale limitation this carries: an org with more meetings than one
// page needs more than one reindex run today.
const reindexPageSize = 100

type ReindexUseCase struct {
	meetingsClient meetings.Client
	embedChunks    *EmbedChunksUseCase
	log            *logger.Logger
}

func NewReindexUseCase(meetingsClient meetings.Client, embedChunks *EmbedChunksUseCase, log *logger.Logger) *ReindexUseCase {
	return &ReindexUseCase{meetingsClient: meetingsClient, embedChunks: embedChunks, log: log}
}

type ReindexResult struct {
	MeetingsReindexed int
	FailedMeetingIDs  []string
}

// Reindex is POST /search/reindex's business logic (admin-only, per
// docs/architecture/api-spec.md — enforced at the gateway and this
// service's own route, not here) — a full-org backfill: enumerate every
// meeting the org has, re-embed each one's chunks. Best-effort per
// meeting: one meeting's embedding failure (e.g. it was never actually
// summarized, so AI Summary Service has no chunks for it) is logged and
// skipped rather than aborting the whole backfill, since "reindex the 40
// meetings that worked" is strictly more useful to an admin than "reindex
// nothing because meeting #7 had no chunks."
func (uc *ReindexUseCase) Reindex(ctx context.Context, orgID string) (*ReindexResult, error) {
	meetingList, _, err := uc.meetingsClient.ListMeetings(ctx, orgID, 1, reindexPageSize)
	if err != nil {
		return nil, err
	}

	result := &ReindexResult{}
	for _, m := range meetingList {
		if _, err := uc.embedChunks.EmbedChunks(ctx, orgID, m.ID); err != nil {
			uc.log.Error("reindex meeting failed", "meeting_id", m.ID, "org_id", orgID, "err", err)
			result.FailedMeetingIDs = append(result.FailedMeetingIDs, m.ID)
			continue
		}
		result.MeetingsReindexed++
	}
	return result, nil
}

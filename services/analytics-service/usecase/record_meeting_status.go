package usecase

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/repository"
)

// meetingStatusCompleted mirrors meetingsvc/entity.StatusCompleted —
// duplicated, not imported, matching every other cross-service constant
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
const meetingStatusCompleted = "completed"

// RecordMeetingStatusChangedUseCase rolls meeting.status-changed.v1 events
// into analytics.meeting_daily_rollup — see
// docs/architecture/microservices.md §11.
type RecordMeetingStatusChangedUseCase struct {
	meetings meetings.Client
	repo     repository.Repository
}

func NewRecordMeetingStatusChangedUseCase(meetings meetings.Client, repo repository.Repository) *RecordMeetingStatusChangedUseCase {
	return &RecordMeetingStatusChangedUseCase{meetings, repo}
}

// RecordMeetingStatusChanged only rolls up a meeting once it reaches
// "completed" — every earlier transition (transcribing, summarizing, ...)
// this same topic also carries is a no-op here, since "meeting_count"/
// "total_minutes" are meant to count finished meetings, not
// in-flight ones.
func (uc *RecordMeetingStatusChangedUseCase) RecordMeetingStatusChanged(ctx context.Context, orgID, meetingID, status string, eventTime time.Time) error {
	if status != meetingStatusCompleted {
		return nil
	}
	minutes, err := uc.meetings.GetDurationMinutes(ctx, orgID, meetingID)
	if err != nil {
		return err
	}
	return uc.repo.UpsertMeetingCompletion(ctx, orgID, dayOf(eventTime), minutes)
}

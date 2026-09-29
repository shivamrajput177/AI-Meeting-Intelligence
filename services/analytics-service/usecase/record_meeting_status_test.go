package usecase

import (
	"context"
	"testing"
	"time"
)

func TestRecordMeetingStatusChanged_Completed(t *testing.T) {
	meetings := &fakeMeetingsClient{durationMinutes: 42}
	repo := &fakeRepository{}
	uc := NewRecordMeetingStatusChangedUseCase(meetings, repo)

	eventTime := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	if err := uc.RecordMeetingStatusChanged(context.Background(), "org-1", "meeting-1", "completed", eventTime); err != nil {
		t.Fatalf("RecordMeetingStatusChanged: %v", err)
	}

	if len(repo.meetingCompletions) != 1 {
		t.Fatalf("expected 1 rollup write, got %d", len(repo.meetingCompletions))
	}
	got := repo.meetingCompletions[0]
	if got.orgID != "org-1" || got.durationMinutes != 42 || !got.day.Equal(dayOf(eventTime)) {
		t.Errorf("unexpected rollup write: %+v", got)
	}
}

func TestRecordMeetingStatusChanged_IgnoresNonCompleted(t *testing.T) {
	meetings := &fakeMeetingsClient{durationMinutes: 42}
	repo := &fakeRepository{}
	uc := NewRecordMeetingStatusChangedUseCase(meetings, repo)

	if err := uc.RecordMeetingStatusChanged(context.Background(), "org-1", "meeting-1", "transcribing", time.Now()); err != nil {
		t.Fatalf("RecordMeetingStatusChanged: %v", err)
	}
	if len(repo.meetingCompletions) != 0 {
		t.Errorf("expected no rollup write for a non-completed status, got %d", len(repo.meetingCompletions))
	}
}

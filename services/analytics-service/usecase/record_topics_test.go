package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/summary"
)

func TestRecordTopicsFromSummary(t *testing.T) {
	summaryC := &fakeSummaryClient{summary: &summary.Summary{
		KeyDecisions: []string{"Ship the API"},
		Risks:        []string{"Vendor delay"},
		Blockers:     nil,
	}}
	repo := &fakeRepository{}
	uc := NewRecordTopicsFromSummaryUseCase(summaryC, repo)

	eventTime := time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC) // a Wednesday
	if err := uc.RecordTopicsFromSummary(context.Background(), "org-1", "meeting-1", eventTime); err != nil {
		t.Fatalf("RecordTopicsFromSummary: %v", err)
	}

	if len(repo.topics) != 2 {
		t.Fatalf("expected 2 topic increments, got %d: %+v", len(repo.topics), repo.topics)
	}
	wantWeek := weekOf(eventTime)
	for _, call := range repo.topics {
		if call.count != 1 || !call.week.Equal(wantWeek) {
			t.Errorf("unexpected topic call: %+v", call)
		}
	}
	if repo.topics[0].topic != "ship the api" || repo.topics[1].topic != "vendor delay" {
		t.Errorf("unexpected topic strings: %+v", repo.topics)
	}
}

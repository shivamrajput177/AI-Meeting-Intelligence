package usecase

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

func TestRequestJiraTicketUseCase_PublishesRequest(t *testing.T) {
	repo := &updateFakeRepository{item: &entity.ActionItem{ID: "item-1", MeetingID: "meeting-1", OrgID: "org-1", Description: "Ship the API", Status: entity.StatusOpen}}
	pub := &updateFakePublisherWithJira{}
	uc := NewRequestJiraTicketUseCase(repo, pub)

	if err := uc.RequestJiraTicket(context.Background(), "org-1", "item-1"); err != nil {
		t.Fatalf("RequestJiraTicket: %v", err)
	}

	if len(pub.jiraRequested) != 1 {
		t.Fatalf("expected 1 jira-requested event, got %d", len(pub.jiraRequested))
	}
	got := pub.jiraRequested[0]
	if got.ActionItemID != "item-1" || got.MeetingID != "meeting-1" || got.OrgID != "org-1" || got.Description != "Ship the API" {
		t.Fatalf("unexpected event: %+v", got)
	}
}

// updateFakePublisherWithJira extends updateFakeRepository's sibling
// fake publisher with a real (not error-returning) PublishActionItemJiraRequested,
// since this test — unlike update_test.go's — actually exercises that call.
type updateFakePublisherWithJira struct {
	updateFakePublisher
	jiraRequested []entity.ActionItemJiraRequestedEvent
}

func (f *updateFakePublisherWithJira) PublishActionItemJiraRequested(_ context.Context, event entity.ActionItemJiraRequestedEvent) error {
	f.jiraRequested = append(f.jiraRequested, event)
	return nil
}

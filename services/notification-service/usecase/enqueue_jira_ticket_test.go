package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

func TestEnqueueJiraTicket(t *testing.T) {
	repo := &fakeRepository{}
	uc := NewEnqueueJiraTicketUseCase(repo)

	if err := uc.EnqueueJiraTicket(context.Background(), "org-1", "item-1", "Ship the API"); err != nil {
		t.Fatalf("EnqueueJiraTicket: %v", err)
	}

	if len(repo.enqueued) != 1 || repo.enqueued[0].channel != entity.ChannelJira {
		t.Fatalf("expected 1 jira outbox row, got %+v", repo.enqueued)
	}
	var payload entity.JiraPayload
	if err := json.Unmarshal(repo.enqueued[0].payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.ActionItemID != "item-1" || payload.Title != "Ship the API" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

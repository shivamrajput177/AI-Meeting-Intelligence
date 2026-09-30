package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

func TestEnqueueReminder_Slack(t *testing.T) {
	repo := &fakeRepository{}
	uc := NewEnqueueReminderUseCase(repo)

	if err := uc.EnqueueReminder(context.Background(), "org-1", entity.ChannelSlack, "Ship the API"); err != nil {
		t.Fatalf("EnqueueReminder: %v", err)
	}

	if len(repo.enqueued) != 1 || repo.enqueued[0].channel != entity.ChannelSlack {
		t.Fatalf("expected 1 slack outbox row, got %+v", repo.enqueued)
	}
	var payload entity.SlackPayload
	if err := json.Unmarshal(repo.enqueued[0].payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Text == "" {
		t.Error("slack payload text is empty")
	}
}

func TestEnqueueReminder_UnsupportedChannel(t *testing.T) {
	repo := &fakeRepository{}
	uc := NewEnqueueReminderUseCase(repo)

	if err := uc.EnqueueReminder(context.Background(), "org-1", entity.ChannelEmail, "Ship the API"); err == nil {
		t.Fatal("expected an error for an unsupported reminder channel")
	}
	if len(repo.enqueued) != 0 {
		t.Fatalf("expected no outbox row enqueued, got %+v", repo.enqueued)
	}
}

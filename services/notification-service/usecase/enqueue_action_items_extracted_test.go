package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings"
)

func TestEnqueueActionItemDigest(t *testing.T) {
	meetingsC := &fakeMeetingsClient{meeting: &meetings.Meeting{Title: "Standup"}}
	repo := &fakeRepository{}
	uc := NewEnqueueActionItemDigestUseCase(meetingsC, repo)

	if err := uc.EnqueueActionItemDigest(context.Background(), "org-1", "meeting-1", 3); err != nil {
		t.Fatalf("EnqueueActionItemDigest: %v", err)
	}

	if len(repo.enqueued) != 1 {
		t.Fatalf("expected 1 enqueued row, got %d", len(repo.enqueued))
	}
	if repo.enqueued[0].orgID != "org-1" || repo.enqueued[0].channel != entity.ChannelSlack {
		t.Errorf("unexpected enqueued row: %+v", repo.enqueued[0])
	}
	var payload entity.SlackPayload
	if err := json.Unmarshal(repo.enqueued[0].payload, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Text == "" {
		t.Error("slack payload text is empty")
	}
}

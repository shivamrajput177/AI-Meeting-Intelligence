package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func TestEnqueueSummaryNotifications_EnqueuesSlackAndEmail(t *testing.T) {
	meetingsC := &fakeMeetingsClient{meeting: &meetings.Meeting{Title: "Q3 Planning", CreatedBy: "user-1"}}
	usersC := &fakeUsersClient{email: "alice@acme.com"}
	repo := &fakeRepository{}
	uc := NewEnqueueSummaryNotificationsUseCase(meetingsC, usersC, repo, logger.New("test", logger.LevelError))

	if err := uc.EnqueueSummaryNotifications(context.Background(), "org-1", "meeting-1"); err != nil {
		t.Fatalf("EnqueueSummaryNotifications: %v", err)
	}

	if len(repo.enqueued) != 2 {
		t.Fatalf("expected 2 enqueued rows, got %d: %+v", len(repo.enqueued), repo.enqueued)
	}
	if repo.enqueued[0].channel != entity.ChannelSlack {
		t.Errorf("first row channel = %q, want %q", repo.enqueued[0].channel, entity.ChannelSlack)
	}
	var slackPayload entity.SlackPayload
	if err := json.Unmarshal(repo.enqueued[0].payload, &slackPayload); err != nil {
		t.Fatalf("decode slack payload: %v", err)
	}
	if slackPayload.Text == "" {
		t.Error("slack payload text is empty")
	}

	if repo.enqueued[1].channel != entity.ChannelEmail {
		t.Errorf("second row channel = %q, want %q", repo.enqueued[1].channel, entity.ChannelEmail)
	}
	var emailPayload entity.EmailPayload
	if err := json.Unmarshal(repo.enqueued[1].payload, &emailPayload); err != nil {
		t.Fatalf("decode email payload: %v", err)
	}
	if emailPayload.To != "alice@acme.com" {
		t.Errorf("email To = %q, want alice@acme.com", emailPayload.To)
	}
}

func TestEnqueueSummaryNotifications_SkipsEmailOnUserLookupFailure(t *testing.T) {
	meetingsC := &fakeMeetingsClient{meeting: &meetings.Meeting{Title: "Q3 Planning", CreatedBy: "user-1"}}
	usersC := &fakeUsersClient{err: errFake}
	repo := &fakeRepository{}
	uc := NewEnqueueSummaryNotificationsUseCase(meetingsC, usersC, repo, logger.New("test", logger.LevelError))

	if err := uc.EnqueueSummaryNotifications(context.Background(), "org-1", "meeting-1"); err != nil {
		t.Fatalf("EnqueueSummaryNotifications: %v", err)
	}

	if len(repo.enqueued) != 1 || repo.enqueued[0].channel != entity.ChannelSlack {
		t.Fatalf("expected only the slack row to be enqueued, got %+v", repo.enqueued)
	}
}

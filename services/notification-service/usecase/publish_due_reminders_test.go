package usecase

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func TestPublishDueReminders_PublishesAndMarksSent(t *testing.T) {
	owner := "user-1"
	repo := newFakeReminderRepository()
	repo.dueRows = []entity.DueReminder{
		{ID: "rem-1", ActionItemID: "item-1", OrgID: "org-1", Description: "Ship the API", OwnerUserID: &owner, Channel: entity.ChannelSlack},
	}
	pub := &fakePublisher{}
	uc := NewPublishDueRemindersUseCase(repo, pub, logger.New("test", logger.LevelError))

	if err := uc.PublishDueReminders(context.Background()); err != nil {
		t.Fatalf("PublishDueReminders: %v", err)
	}

	if !repo.lockCalled {
		t.Fatal("expected WithLeaderLock to be called")
	}
	if len(pub.remindersDue) != 1 || pub.remindersDue[0].ActionItemID != "item-1" {
		t.Fatalf("expected 1 reminder-due event for item-1, got %+v", pub.remindersDue)
	}
	if len(repo.sent) != 1 || repo.sent[0] != "rem-1" {
		t.Fatalf("expected rem-1 marked sent, got %v", repo.sent)
	}
}

func TestPublishDueReminders_SkipsWhenLockNotAcquired(t *testing.T) {
	repo := newFakeReminderRepository()
	repo.lockAcquired = false
	repo.dueRows = []entity.DueReminder{{ID: "rem-1", ActionItemID: "item-1", OrgID: "org-1", Channel: entity.ChannelSlack}}
	pub := &fakePublisher{}
	uc := NewPublishDueRemindersUseCase(repo, pub, logger.New("test", logger.LevelError))

	if err := uc.PublishDueReminders(context.Background()); err != nil {
		t.Fatalf("PublishDueReminders: %v", err)
	}

	if len(pub.remindersDue) != 0 {
		t.Fatalf("expected no publishes when another replica holds the lock, got %+v", pub.remindersDue)
	}
	if len(repo.sent) != 0 {
		t.Fatalf("expected nothing marked sent, got %v", repo.sent)
	}
}

func TestPublishDueReminders_LeavesUnpublishedOnPublishFailure(t *testing.T) {
	repo := newFakeReminderRepository()
	repo.dueRows = []entity.DueReminder{{ID: "rem-1", ActionItemID: "item-1", OrgID: "org-1", Channel: entity.ChannelSlack}}
	pub := &fakePublisher{reminderErr: errFake}
	uc := NewPublishDueRemindersUseCase(repo, pub, logger.New("test", logger.LevelError))

	if err := uc.PublishDueReminders(context.Background()); err != nil {
		t.Fatalf("PublishDueReminders: %v", err)
	}

	if len(repo.sent) != 0 {
		t.Fatalf("expected rem-1 left unsent for the next tick to retry, got %v", repo.sent)
	}
}

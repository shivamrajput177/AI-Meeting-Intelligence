package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// updateFakeRepository is a minimal in-memory Repository stand-in scoped to
// what UpdateActionItemUseCase actually calls (GetByID, Update) — separate
// from extract_test.go's fakeRepository since that one deliberately returns
// "not implemented in fake" for both.
type updateFakeRepository struct {
	item   *entity.ActionItem
	update *entity.ActionItem
}

func (f *updateFakeRepository) ReplaceActionItems(context.Context, string, string, []*entity.ActionItem) error {
	return errors.New("not implemented in fake")
}
func (f *updateFakeRepository) GetByID(_ context.Context, _, _ string) (*entity.ActionItem, error) {
	if f.item == nil {
		return nil, apperr.NotFound("action item not found")
	}
	return f.item, nil
}
func (f *updateFakeRepository) ListByMeeting(context.Context, string, string) ([]*entity.ActionItem, error) {
	return nil, errors.New("not implemented in fake")
}
func (f *updateFakeRepository) List(context.Context, string, entity.ListActionItemsFilter) ([]*entity.ActionItem, int, error) {
	return nil, 0, errors.New("not implemented in fake")
}
func (f *updateFakeRepository) Update(_ context.Context, _, _ string, _ entity.UpdateActionItemInput) (*entity.ActionItem, error) {
	return f.update, nil
}

type updateFakePublisher struct {
	mu            sync.Mutex
	statusChanged []entity.ActionItemStatusChangedEvent
}

func (f *updateFakePublisher) PublishActionItemExtracted(context.Context, entity.ActionItemExtractedEvent) error {
	return errors.New("not implemented in fake")
}
func (f *updateFakePublisher) PublishActionItemExtractionFailed(context.Context, entity.ActionItemExtractionFailedEvent) error {
	return errors.New("not implemented in fake")
}
func (f *updateFakePublisher) PublishActionItemStatusChanged(_ context.Context, event entity.ActionItemStatusChangedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusChanged = append(f.statusChanged, event)
	return nil
}
func (f *updateFakePublisher) PublishActionItemJiraRequested(context.Context, entity.ActionItemJiraRequestedEvent) error {
	return errors.New("not implemented in fake")
}

func ownerPtr(s string) *string { return &s }

func TestUpdateActionItemUseCase_PublishesOnStatusChange(t *testing.T) {
	owner := "user-1"
	repo := &updateFakeRepository{
		item:   &entity.ActionItem{ID: "item-1", MeetingID: "meeting-1", OrgID: "org-1", Status: entity.StatusOpen, OwnerUserID: &owner},
		update: &entity.ActionItem{ID: "item-1", MeetingID: "meeting-1", OrgID: "org-1", Status: entity.StatusDone, OwnerUserID: &owner},
	}
	pub := &updateFakePublisher{}
	uc := NewUpdateActionItemUseCase(repo, pub, logger.New("test", logger.LevelError))

	done := entity.StatusDone
	_, err := uc.UpdateActionItem(context.Background(), "org-1", "item-1", "user-1", "member", entity.UpdateActionItemInput{Status: &done})
	if err != nil {
		t.Fatalf("UpdateActionItem: %v", err)
	}

	if len(pub.statusChanged) != 1 {
		t.Fatalf("expected 1 status-changed event, got %d", len(pub.statusChanged))
	}
	got := pub.statusChanged[0]
	if got.ActionItemID != "item-1" || got.Status != entity.StatusDone || got.OrgID != "org-1" {
		t.Fatalf("unexpected event: %+v", got)
	}
}

func TestUpdateActionItemUseCase_NoPublishWhenStatusUnchanged(t *testing.T) {
	owner := "user-1"
	repo := &updateFakeRepository{
		item:   &entity.ActionItem{ID: "item-1", MeetingID: "meeting-1", OrgID: "org-1", Status: entity.StatusOpen, OwnerUserID: &owner},
		update: &entity.ActionItem{ID: "item-1", MeetingID: "meeting-1", OrgID: "org-1", Status: entity.StatusOpen, OwnerUserID: ownerPtr("user-2")},
	}
	pub := &updateFakePublisher{}
	uc := NewUpdateActionItemUseCase(repo, pub, logger.New("test", logger.LevelError))

	_, err := uc.UpdateActionItem(context.Background(), "org-1", "item-1", "user-1", "member", entity.UpdateActionItemInput{OwnerUserID: ownerPtr("user-2")})
	if err != nil {
		t.Fatalf("UpdateActionItem: %v", err)
	}

	if len(pub.statusChanged) != 0 {
		t.Fatalf("expected no status-changed event, got %d", len(pub.statusChanged))
	}
}

func TestUpdateActionItemUseCase_ForbiddenForUnrelatedMember(t *testing.T) {
	owner := "user-1"
	repo := &updateFakeRepository{item: &entity.ActionItem{ID: "item-1", OrgID: "org-1", Status: entity.StatusOpen, OwnerUserID: &owner}}
	pub := &updateFakePublisher{}
	uc := NewUpdateActionItemUseCase(repo, pub, logger.New("test", logger.LevelError))

	done := entity.StatusDone
	_, err := uc.UpdateActionItem(context.Background(), "org-1", "item-1", "user-2", "member", entity.UpdateActionItemInput{Status: &done})
	if err == nil {
		t.Fatal("expected forbidden error, got nil")
	}
}

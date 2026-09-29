package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/actionitems"
)

func TestRecordActionItemsExtracted_SkipsUnowned(t *testing.T) {
	owner1 := "user-1"
	items := &fakeActionItemsClient{items: []actionitems.Item{
		{ID: "item-1", OwnerUserID: &owner1},
		{ID: "item-2", OwnerUserID: nil},
	}}
	repo := &fakeRepository{}
	uc := NewRecordActionItemsExtractedUseCase(items, repo)

	eventTime := time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC)
	if err := uc.RecordActionItemsExtracted(context.Background(), "org-1", "meeting-1", eventTime); err != nil {
		t.Fatalf("RecordActionItemsExtracted: %v", err)
	}

	if len(repo.opened) != 1 {
		t.Fatalf("expected 1 opened increment, got %d", len(repo.opened))
	}
	if repo.opened[0].ownerUserID != owner1 || !repo.opened[0].day.Equal(dayOf(eventTime)) {
		t.Errorf("unexpected opened increment: %+v", repo.opened[0])
	}
}

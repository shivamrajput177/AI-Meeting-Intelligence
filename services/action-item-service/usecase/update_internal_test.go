package usecase

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

func TestUpdateActionItemInternalUseCase_SetsJiraIssueKey(t *testing.T) {
	updated := &entity.ActionItem{ID: "item-1", OrgID: "org-1", Status: entity.StatusOpen}
	repo := &updateFakeRepository{
		item:   &entity.ActionItem{ID: "item-1", OrgID: "org-1", Status: entity.StatusOpen},
		update: updated,
	}
	uc := NewUpdateActionItemInternalUseCase(repo)

	key := "DEMO-1"
	got, err := uc.UpdateActionItemInternal(context.Background(), "org-1", "item-1", entity.UpdateActionItemInput{JiraIssueKey: &key})
	if err != nil {
		t.Fatalf("UpdateActionItemInternal: %v", err)
	}
	if got != updated {
		t.Fatalf("expected the repo's own updated row to be returned unchanged")
	}
}

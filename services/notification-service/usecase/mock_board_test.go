package usecase

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func TestGetMockBoard(t *testing.T) {
	repo := &fakeJiraRepository{board: []entity.MockJiraIssue{{IssueKey: "DEMO-1", Title: "Ship the API", Status: entity.MockStatusToDo}}}
	uc := NewGetMockBoardUseCase(repo)

	issues, err := uc.GetBoard(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("GetBoard: %v", err)
	}
	if len(issues) != 1 || issues[0].IssueKey != "DEMO-1" {
		t.Fatalf("unexpected board: %+v", issues)
	}
}

func TestTransitionMockIssue_WritesBackMappedStatus(t *testing.T) {
	repo := &fakeJiraRepository{transitionResult: "item-1"}
	actionItems := &fakeActionItemsClient{}
	uc := NewTransitionMockIssueUseCase(repo, actionItems, logger.New("test", logger.LevelError))

	if err := uc.TransitionMockIssue(context.Background(), "org-1", "DEMO-1", entity.MockStatusDone); err != nil {
		t.Fatalf("TransitionMockIssue: %v", err)
	}

	if len(repo.transitions) != 1 || repo.transitions[0].status != entity.MockStatusDone {
		t.Fatalf("expected a transition to Done, got %+v", repo.transitions)
	}
	if len(actionItems.updates) != 1 || actionItems.updates[0].actionItemID != "item-1" ||
		actionItems.updates[0].status == nil || *actionItems.updates[0].status != "done" {
		t.Fatalf("expected the linked action item's status written back to done, got %+v", actionItems.updates)
	}
}

func TestTransitionMockIssue_RejectsInvalidStatus(t *testing.T) {
	repo := &fakeJiraRepository{}
	uc := NewTransitionMockIssueUseCase(repo, &fakeActionItemsClient{}, logger.New("test", logger.LevelError))

	if err := uc.TransitionMockIssue(context.Background(), "org-1", "DEMO-1", "Blocked"); err == nil {
		t.Fatal("expected an error for an invalid mock status")
	}
	if len(repo.transitions) != 0 {
		t.Fatalf("expected no transition attempted for an invalid status, got %+v", repo.transitions)
	}
}

func TestTransitionMockIssue_WriteBackFailureDoesNotFail(t *testing.T) {
	repo := &fakeJiraRepository{transitionResult: "item-1"}
	actionItems := &fakeActionItemsClient{err: errFake}
	uc := NewTransitionMockIssueUseCase(repo, actionItems, logger.New("test", logger.LevelError))

	if err := uc.TransitionMockIssue(context.Background(), "org-1", "DEMO-1", entity.MockStatusInProgress); err != nil {
		t.Fatalf("TransitionMockIssue: %v", err)
	}
}

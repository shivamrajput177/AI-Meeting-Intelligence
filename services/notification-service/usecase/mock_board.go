package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/actionitems"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// GetMockBoardUseCase backs both board reads (GET /demo/board, GET
// /orgs/{orgId}/mock-jira/board) — see
// docs/architecture/deployment-demo-strategy.md §3.
type GetMockBoardUseCase struct {
	repo repository.JiraRepository
}

func NewGetMockBoardUseCase(repo repository.JiraRepository) *GetMockBoardUseCase {
	return &GetMockBoardUseCase{repo}
}

func (uc *GetMockBoardUseCase) GetBoard(ctx context.Context, orgID string) ([]entity.MockJiraIssue, error) {
	return uc.repo.ListMockBoard(ctx, orgID)
}

// TransitionMockIssueUseCase is PATCH
// /orgs/{orgId}/mock-jira/issues/{issueKey}'s business logic — a manual
// column move that also writes the mapped status back onto the linked
// action item, "the same status-changed event a real Jira transition
// would" fire (deployment-demo-strategy.md §3).
type TransitionMockIssueUseCase struct {
	repo        repository.JiraRepository
	actionItems actionitems.Client
	log         *logger.Logger
}

func NewTransitionMockIssueUseCase(repo repository.JiraRepository, actionItems actionitems.Client, log *logger.Logger) *TransitionMockIssueUseCase {
	return &TransitionMockIssueUseCase{repo, actionItems, log}
}

func (uc *TransitionMockIssueUseCase) TransitionMockIssue(ctx context.Context, orgID, issueKey, newStatus string) error {
	if !entity.ValidMockStatuses[newStatus] {
		return apperr.BadRequest("invalid status")
	}
	actionItemID, err := uc.repo.TransitionMockIssue(ctx, orgID, issueKey, newStatus)
	if err != nil {
		return err
	}

	// Best-effort write-back — the board transition itself already
	// succeeded above, so a failure here (e.g. Action Item Service is
	// down) is logged, not surfaced as a 500 for what the caller sees as
	// a successful card move.
	mappedStatus, _ := mapMockStatusToActionItemStatus(newStatus) // newStatus already validated against ValidMockStatuses above
	if err := uc.actionItems.UpdateActionItem(ctx, orgID, actionItemID, &mappedStatus, nil); err != nil {
		uc.log.Error("write back action item status from mock board transition", "action_item_id", actionItemID, "issue_key", issueKey, "err", err)
	}
	return nil
}

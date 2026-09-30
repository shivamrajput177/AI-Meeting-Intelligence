package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository"
)

// RequestJiraTicketUseCase is POST /action-items/{id}/jira-ticket's
// business logic — see docs/architecture/kafka-topics.md's flow-3
// diagram: this call only publishes the request and returns; the actual
// ticket (mock or real, per the org's configured TicketProvider) is
// created asynchronously by Notification Service, which writes the
// resulting jira_issue_key back via PATCH /internal/action-items/{id}.
type RequestJiraTicketUseCase struct {
	repo      repository.Repository
	publisher events.Publisher
}

func NewRequestJiraTicketUseCase(repo repository.Repository, publisher events.Publisher) *RequestJiraTicketUseCase {
	return &RequestJiraTicketUseCase{repo, publisher}
}

func (uc *RequestJiraTicketUseCase) RequestJiraTicket(ctx context.Context, orgID, id string) error {
	item, err := uc.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}
	return uc.publisher.PublishActionItemJiraRequested(ctx, entity.ActionItemJiraRequestedEvent{
		ActionItemID: item.ID, MeetingID: item.MeetingID, OrgID: orgID, Description: item.Description,
	})
}

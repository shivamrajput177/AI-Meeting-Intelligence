package usecase

import (
	"context"
	"encoding/json"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
)

// EnqueueJiraTicketUseCase turns action-item.jira-requested.v1 into a
// jira outbox row — see docs/architecture/kafka-topics.md's flow-3
// diagram: the actual ticket (mock or real) is created later by
// DispatchUseCase's poller, not here — this is only the "durable write"
// half of the outbox pattern (see entity.OutboxRow's doc comment).
type EnqueueJiraTicketUseCase struct {
	repo repository.Repository
}

func NewEnqueueJiraTicketUseCase(repo repository.Repository) *EnqueueJiraTicketUseCase {
	return &EnqueueJiraTicketUseCase{repo}
}

func (uc *EnqueueJiraTicketUseCase) EnqueueJiraTicket(ctx context.Context, orgID, actionItemID, description string) error {
	payload, err := json.Marshal(entity.JiraPayload{ActionItemID: actionItemID, Title: description})
	if err != nil {
		return err
	}
	return uc.repo.Enqueue(ctx, orgID, entity.ChannelJira, payload)
}

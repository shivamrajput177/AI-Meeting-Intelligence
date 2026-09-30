package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
)

// EnqueueReminderUseCase is the "dispatcher" half of the reminder
// scheduler (see PublishDueRemindersUseCase's own doc comment): it
// self-consumes action-item.reminder-due.v1 and enqueues an outbox row —
// the same transactional-outbox/poller machinery Phase 4.1 already built
// for every other channel, reused here unchanged.
type EnqueueReminderUseCase struct {
	repo repository.Repository
}

func NewEnqueueReminderUseCase(repo repository.Repository) *EnqueueReminderUseCase {
	return &EnqueueReminderUseCase{repo}
}

// EnqueueReminder only knows how to route a "slack" reminder today — see
// entity.DueReminder's own Channel field: nothing in this codebase ever
// creates an actionitem.reminders row with a different channel value yet
// (action-item-service's extraction pipeline always writes 'slack', the
// column's own default), so any other value is a real configuration
// error to surface now rather than a case worth building speculative
// dispatch logic for.
func (uc *EnqueueReminderUseCase) EnqueueReminder(ctx context.Context, orgID, channel, description string) error {
	if channel != entity.ChannelSlack {
		return fmt.Errorf("reminder channel %q not supported yet", channel)
	}
	payload, err := json.Marshal(entity.SlackPayload{Text: renderReminderSlackText(description)})
	if err != nil {
		return err
	}
	return uc.repo.Enqueue(ctx, orgID, entity.ChannelSlack, payload)
}

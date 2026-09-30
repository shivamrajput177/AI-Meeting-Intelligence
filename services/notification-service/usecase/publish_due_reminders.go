package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// reminderBatchSize caps how many due reminders one leader tick claims —
// matching DispatchUseCase's own pollerBatchSize-style bound (main.go's
// poller ticks this every 60s per
// docs/architecture/microservices.md §10, so a batch this size clears
// comfortably within one tick even on a slow Postgres).
const reminderBatchSize = 50

// PublishDueRemindersUseCase is the "leader" half of the reminder
// scheduler (docs/architecture/microservices.md §10, kafka-topics.md's
// flow-4 diagram): it runs only on whichever replica's tick wins the
// advisory lock (see repository.ReminderRepository.WithLeaderLock),
// finds every reminder past its remind_at with no sent_at yet, and
// publishes action-item.reminder-due.v1 for each — the actual dispatch
// (Slack message) happens later, in this same service's own
// EnqueueReminderUseCase, self-consuming that same topic.
type PublishDueRemindersUseCase struct {
	repo      repository.ReminderRepository
	publisher events.Publisher
	log       *logger.Logger
}

func NewPublishDueRemindersUseCase(repo repository.ReminderRepository, publisher events.Publisher, log *logger.Logger) *PublishDueRemindersUseCase {
	return &PublishDueRemindersUseCase{repo, publisher, log}
}

// PublishDueReminders is a no-op, successful call on every tick this
// replica doesn't win leadership for — see WithLeaderLock's own doc
// comment on why that's the expected common case, not an error.
func (uc *PublishDueRemindersUseCase) PublishDueReminders(ctx context.Context) error {
	return uc.repo.WithLeaderLock(ctx, func(ctx context.Context) error {
		reminders, err := uc.repo.ClaimDueReminders(ctx, reminderBatchSize)
		if err != nil {
			return err
		}
		for _, rem := range reminders {
			if err := uc.publisher.PublishActionItemReminderDue(ctx, entity.ActionItemReminderDueEvent{
				ActionItemID: rem.ActionItemID, OrgID: rem.OrgID, Description: rem.Description,
				OwnerUserID: rem.OwnerUserID, Channel: rem.Channel,
			}); err != nil {
				// Not marked sent: left for the next tick (whichever
				// replica wins it) to retry — the same "durable until
				// actually delivered" guarantee the outbox pattern gives
				// Slack/email/Jira dispatch elsewhere in this service.
				uc.log.Error("publish action-item.reminder-due.v1", "reminder_id", rem.ID, "action_item_id", rem.ActionItemID, "err", err)
				continue
			}
			if err := uc.repo.MarkReminderSent(ctx, rem.ID); err != nil {
				uc.log.Error("mark reminder sent", "reminder_id", rem.ID, "err", err)
			}
		}
		return nil
	})
}

// scheduler.go ticks PublishDueRemindersUseCase — the leader-elected half
// of the reminder scheduler (see poller.go's own doc comment on why this
// one, unlike the outbox dispatcher, needs leader election rather than
// running safely on every replica: the Kafka publish and the
// mark-sent update aren't atomic with each other, so having more than one
// replica active at once risks double-publishing the same reminder).
package main

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// reminderSchedulerInterval matches
// docs/architecture/microservices.md §10's own "ticking every 60s".
const reminderSchedulerInterval = 60 * time.Second

// RunReminderScheduler runs until ctx is cancelled, trying once per tick
// to win this service's leader lock and publish whatever reminders are
// due — every replica runs this same loop; WithLeaderLock is what makes
// only one of them actually do anything on a given tick.
func RunReminderScheduler(ctx context.Context, publishDue *usecase.PublishDueRemindersUseCase, log *logger.Logger) {
	ticker := time.NewTicker(reminderSchedulerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := publishDue.PublishDueReminders(ctx); err != nil {
				log.Error("publish due reminders", "err", err)
			}
		}
	}
}

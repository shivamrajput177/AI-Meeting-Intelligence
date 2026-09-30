package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/email"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/slack"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// DispatchUseCase is the poller half of the transactional outbox pattern
// (see entity.OutboxRow's doc comment): ClaimBatch durably marks a batch
// of rows "in flight" so no two poller replicas double-send the same one,
// then each row is dispatched to its channel's real external call.
type DispatchUseCase struct {
	repo            repository.Repository
	slack           slack.Sender
	email           email.Sender
	publisher       events.Publisher
	log             *logger.Logger
	slackWebhookURL string
}

func NewDispatchUseCase(repo repository.Repository, slackSender slack.Sender, emailSender email.Sender, publisher events.Publisher, log *logger.Logger, slackWebhookURL string) *DispatchUseCase {
	return &DispatchUseCase{repo, slackSender, emailSender, publisher, log, slackWebhookURL}
}

// DispatchBatch claims up to batchSize eligible rows and attempts each —
// every row's outcome (sent, retried, or permanently failed) is handled
// independently, so one bad row never blocks the rest of the batch.
func (uc *DispatchUseCase) DispatchBatch(ctx context.Context, batchSize int) (int, error) {
	rows, err := uc.repo.ClaimBatch(ctx, batchSize)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		uc.dispatchOne(ctx, row)
	}
	return len(rows), nil
}

func (uc *DispatchUseCase) dispatchOne(ctx context.Context, row entity.OutboxRow) {
	if err := uc.send(ctx, row); err != nil {
		attempts := row.Attempts + 1
		permanent := shouldGiveUp(attempts)
		if err := uc.repo.MarkAttemptFailed(ctx, row.ID, attempts, err.Error(), permanent); err != nil {
			uc.log.Error("mark attempt failed", "outbox_id", row.ID, "err", err)
		}
		if permanent {
			uc.log.Error("giving up on notification", "outbox_id", row.ID, "channel", row.Channel, "attempts", attempts, "err", err)
			if pubErr := uc.publisher.PublishNotificationFailed(ctx, entity.NotificationFailedEvent{
				OutboxID: row.ID, OrgID: row.OrgID, Channel: row.Channel, Reason: err.Error(),
			}); pubErr != nil {
				uc.log.Error("publish notification.failed.v1", "outbox_id", row.ID, "err", pubErr)
			}
		}
		return
	}

	if err := uc.repo.MarkSent(ctx, row.ID); err != nil {
		uc.log.Error("mark sent", "outbox_id", row.ID, "err", err)
		return
	}
	if err := uc.publisher.PublishNotificationSent(ctx, entity.NotificationSentEvent{
		OutboxID: row.ID, OrgID: row.OrgID, Channel: row.Channel,
	}); err != nil {
		uc.log.Error("publish notification.sent.v1", "outbox_id", row.ID, "err", err)
	}
}

// send dispatches row to its real external channel — the one place this
// service actually talks to Slack/SMTP.
func (uc *DispatchUseCase) send(ctx context.Context, row entity.OutboxRow) error {
	switch row.Channel {
	case entity.ChannelSlack:
		var p entity.SlackPayload
		if err := json.Unmarshal(row.Payload, &p); err != nil {
			return fmt.Errorf("decode slack payload: %w", err)
		}
		return uc.slack.Send(ctx, uc.slackWebhookURL, p.Text)
	case entity.ChannelEmail:
		var p entity.EmailPayload
		if err := json.Unmarshal(row.Payload, &p); err != nil {
			return fmt.Errorf("decode email payload: %w", err)
		}
		return uc.email.Send(ctx, p.To, p.Subject, p.Body)
	case entity.ChannelJira:
		// Phase 4.2/4.3's job — nothing enqueues a jira row yet (see
		// migrations/0001_init.up.sql's own comment), so this is
		// unreachable today, not a stub standing in for real logic.
		return fmt.Errorf("jira dispatch not implemented until Phase 4.2/4.3")
	default:
		return fmt.Errorf("unknown channel %q", row.Channel)
	}
}

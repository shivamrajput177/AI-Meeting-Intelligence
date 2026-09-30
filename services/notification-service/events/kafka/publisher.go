// Package kafka implements events.Publisher against Kafka — see
// docs/architecture/kafka-topics.md for topic naming/partitioning
// conventions. notification.sent.v1/notification.failed.v1 were
// documented there since this service's own design (Phase 4's
// predecessor rows in that table) but never actually published until now
// — the same kind of gap Phase 2.6 found and fixed for
// meeting.status-changed.v1.
package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
)

const (
	TopicNotificationSent      = "notification.sent.v1"
	TopicNotificationFailed    = "notification.failed.v1"
	TopicActionItemReminderDue = "action-item.reminder-due.v1"
)

type Publisher struct {
	notificationSent      *kafkago.Writer
	notificationFailed    *kafkago.Writer
	actionItemReminderDue *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		notificationSent:      kafkax.NewWriter(brokers, TopicNotificationSent),
		notificationFailed:    kafkax.NewWriter(brokers, TopicNotificationFailed),
		actionItemReminderDue: kafkax.NewWriter(brokers, TopicActionItemReminderDue),
	}
}

func (p *Publisher) Close() error {
	if err := p.notificationSent.Close(); err != nil {
		return err
	}
	if err := p.notificationFailed.Close(); err != nil {
		return err
	}
	return p.actionItemReminderDue.Close()
}

// PublishNotificationSent/Failed key by org_id, per kafka-topics.md's
// topic catalog — a delivery audit/failure signal is an org-wide event,
// not scoped to one meeting or action item the way this repo's other
// topics are. PublishActionItemReminderDue keys by action_item_id
// instead, matching that topic's own catalog row.

func (p *Publisher) PublishNotificationSent(ctx context.Context, event entity.NotificationSentEvent) error {
	return kafkax.Publish(ctx, p.notificationSent, event.OrgID, event)
}

func (p *Publisher) PublishNotificationFailed(ctx context.Context, event entity.NotificationFailedEvent) error {
	return kafkax.Publish(ctx, p.notificationFailed, event.OrgID, event)
}

func (p *Publisher) PublishActionItemReminderDue(ctx context.Context, event entity.ActionItemReminderDueEvent) error {
	return kafkax.Publish(ctx, p.actionItemReminderDue, event.ActionItemID, event)
}

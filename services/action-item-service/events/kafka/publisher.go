// Package kafka implements events.Publisher against Kafka — see
// docs/architecture/kafka-topics.md for topic naming/partitioning
// conventions and the event-flow diagram this is part of
// ("AC->>K: action-item.extracted.v1").
package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
)

const (
	TopicActionItemExtracted        = "action-item.extracted.v1"
	TopicActionItemExtractionFailed = "action-item.extraction-failed.v1"
	TopicActionItemStatusChanged    = "action-item.status-changed.v1"
)

type Publisher struct {
	actionItemExtracted        *kafkago.Writer
	actionItemExtractionFailed *kafkago.Writer
	actionItemStatusChanged    *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		actionItemExtracted:        kafkax.NewWriter(brokers, TopicActionItemExtracted),
		actionItemExtractionFailed: kafkax.NewWriter(brokers, TopicActionItemExtractionFailed),
		actionItemStatusChanged:    kafkax.NewWriter(brokers, TopicActionItemStatusChanged),
	}
}

func (p *Publisher) Close() error {
	if err := p.actionItemExtracted.Close(); err != nil {
		return err
	}
	if err := p.actionItemExtractionFailed.Close(); err != nil {
		return err
	}
	return p.actionItemStatusChanged.Close()
}

// Every Publish* call below keys per kafka-topics.md's topic catalog:
// meeting_id for extraction events ("per-meeting ordering matters"),
// action_item_id for status-changed (that topic's own documented key,
// since a single action item's own status transitions are what need to
// stay ordered, not a whole meeting's).

func (p *Publisher) PublishActionItemExtracted(ctx context.Context, event entity.ActionItemExtractedEvent) error {
	return kafkax.Publish(ctx, p.actionItemExtracted, event.MeetingID, event)
}

func (p *Publisher) PublishActionItemExtractionFailed(ctx context.Context, event entity.ActionItemExtractionFailedEvent) error {
	return kafkax.Publish(ctx, p.actionItemExtractionFailed, event.MeetingID, event)
}

func (p *Publisher) PublishActionItemStatusChanged(ctx context.Context, event entity.ActionItemStatusChangedEvent) error {
	return kafkax.Publish(ctx, p.actionItemStatusChanged, event.ActionItemID, event)
}

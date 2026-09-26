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
)

type Publisher struct {
	actionItemExtracted        *kafkago.Writer
	actionItemExtractionFailed *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		actionItemExtracted:        kafkax.NewWriter(brokers, TopicActionItemExtracted),
		actionItemExtractionFailed: kafkax.NewWriter(brokers, TopicActionItemExtractionFailed),
	}
}

func (p *Publisher) Close() error {
	if err := p.actionItemExtracted.Close(); err != nil {
		return err
	}
	return p.actionItemExtractionFailed.Close()
}

// Every Publish* call below keys by meeting_id, per kafka-topics.md's
// "per-meeting ordering matters" rule.

func (p *Publisher) PublishActionItemExtracted(ctx context.Context, event entity.ActionItemExtractedEvent) error {
	return kafkax.Publish(ctx, p.actionItemExtracted, event.MeetingID, event)
}

func (p *Publisher) PublishActionItemExtractionFailed(ctx context.Context, event entity.ActionItemExtractionFailedEvent) error {
	return kafkax.Publish(ctx, p.actionItemExtractionFailed, event.MeetingID, event)
}

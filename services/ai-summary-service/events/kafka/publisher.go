// Package kafka implements events.Publisher against Kafka — see
// docs/architecture/kafka-topics.md for topic naming/partitioning
// conventions and the event-flow diagram this is the middle of
// ("AI->>K: chunk.created.v1", "AI->>K: summary.completed.v1").
package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
)

const (
	TopicChunkCreated     = "chunk.created.v1"
	TopicSummaryCompleted = "summary.completed.v1"
	TopicSummaryFailed    = "summary.failed.v1"
)

type Publisher struct {
	chunkCreated     *kafkago.Writer
	summaryCompleted *kafkago.Writer
	summaryFailed    *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		chunkCreated:     kafkax.NewWriter(brokers, TopicChunkCreated),
		summaryCompleted: kafkax.NewWriter(brokers, TopicSummaryCompleted),
		summaryFailed:    kafkax.NewWriter(brokers, TopicSummaryFailed),
	}
}

func (p *Publisher) Close() error {
	if err := p.chunkCreated.Close(); err != nil {
		return err
	}
	if err := p.summaryCompleted.Close(); err != nil {
		return err
	}
	return p.summaryFailed.Close()
}

// Every Publish* call below keys by meeting_id, per kafka-topics.md's
// "per-meeting ordering matters" rule.

func (p *Publisher) PublishChunkCreated(ctx context.Context, event entity.ChunkCreatedEvent) error {
	return kafkax.Publish(ctx, p.chunkCreated, event.MeetingID, event)
}

func (p *Publisher) PublishSummaryCompleted(ctx context.Context, event entity.SummaryCompletedEvent) error {
	return kafkax.Publish(ctx, p.summaryCompleted, event.MeetingID, event)
}

func (p *Publisher) PublishSummaryFailed(ctx context.Context, event entity.SummaryFailedEvent) error {
	return kafkax.Publish(ctx, p.summaryFailed, event.MeetingID, event)
}

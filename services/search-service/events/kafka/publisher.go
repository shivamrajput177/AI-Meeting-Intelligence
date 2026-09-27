// Package kafka implements events.Publisher against Kafka — see
// docs/architecture/kafka-topics.md for topic naming/partitioning
// conventions and the event-flow diagram this is part of
// ("SE->>SE: Ollama embedding -> pgvector").
package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
)

const (
	TopicEmbeddingCompleted = "embedding.completed.v1"
	TopicEmbeddingFailed    = "embedding.failed.v1"
)

type Publisher struct {
	embeddingCompleted *kafkago.Writer
	embeddingFailed    *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		embeddingCompleted: kafkax.NewWriter(brokers, TopicEmbeddingCompleted),
		embeddingFailed:    kafkax.NewWriter(brokers, TopicEmbeddingFailed),
	}
}

func (p *Publisher) Close() error {
	if err := p.embeddingCompleted.Close(); err != nil {
		return err
	}
	return p.embeddingFailed.Close()
}

// Every Publish* call below keys by meeting_id, per kafka-topics.md's
// "per-meeting ordering matters" rule.

func (p *Publisher) PublishEmbeddingCompleted(ctx context.Context, event entity.EmbeddingCompletedEvent) error {
	return kafkax.Publish(ctx, p.embeddingCompleted, event.MeetingID, event)
}

func (p *Publisher) PublishEmbeddingFailed(ctx context.Context, event entity.EmbeddingFailedEvent) error {
	return kafkax.Publish(ctx, p.embeddingFailed, event.MeetingID, event)
}

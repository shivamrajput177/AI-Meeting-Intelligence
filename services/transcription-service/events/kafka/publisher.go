// Package kafka implements events.Publisher against Kafka — see
// docs/architecture/kafka-topics.md for topic naming/partitioning
// conventions and the event-flow diagram this is steps two/two-failed of
// ("TR->>K: transcription.completed.v1").
package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
)

const (
	TopicTranscriptionCompleted = "transcription.completed.v1"
	TopicTranscriptionFailed    = "transcription.failed.v1"
)

type Publisher struct {
	completed *kafkago.Writer
	failed    *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		completed: kafkax.NewWriter(brokers, TopicTranscriptionCompleted),
		failed:    kafkax.NewWriter(brokers, TopicTranscriptionFailed),
	}
}

func (p *Publisher) Close() error {
	if err := p.completed.Close(); err != nil {
		return err
	}
	return p.failed.Close()
}

// PublishTranscriptionCompleted/Failed both key by meeting_id, per
// kafka-topics.md's "per-meeting ordering matters" rule.
func (p *Publisher) PublishTranscriptionCompleted(ctx context.Context, event entity.TranscriptionCompletedEvent) error {
	return kafkax.Publish(ctx, p.completed, event.MeetingID, event)
}

func (p *Publisher) PublishTranscriptionFailed(ctx context.Context, event entity.TranscriptionFailedEvent) error {
	return kafkax.Publish(ctx, p.failed, event.MeetingID, event)
}

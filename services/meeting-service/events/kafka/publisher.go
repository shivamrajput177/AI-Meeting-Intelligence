// Package kafka implements events.Publisher against Kafka — see
// docs/architecture/kafka-topics.md for topic naming/partitioning
// conventions and the event-flow diagram this is step one of ("MS->>K:
// meeting.uploaded.v1").
package kafka

import (
	"context"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
)

// TopicMeetingUploaded is meeting.uploaded.v1 — see kafka-topics.md's
// topic catalog for its consumer (Transcription Service), key
// (meeting_id), partitions, and retention.
const TopicMeetingUploaded = "meeting.uploaded.v1"

type Publisher struct {
	writer *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{writer: kafkax.NewWriter(brokers, TopicMeetingUploaded)}
}

func (p *Publisher) Close() error { return p.writer.Close() }

// PublishMeetingUploaded keys the message by meeting_id, per
// kafka-topics.md's "per-meeting ordering matters" rule — a meeting's own
// event sequence stays ordered within its partition even though this
// service only ever publishes this one event per meeting today.
func (p *Publisher) PublishMeetingUploaded(ctx context.Context, event entity.MeetingUploadedEvent) error {
	return kafkax.Publish(ctx, p.writer, event.MeetingID, event)
}

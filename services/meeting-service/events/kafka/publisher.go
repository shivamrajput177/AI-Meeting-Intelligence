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

// Topic names — see kafka-topics.md's topic catalog for consumers, key
// (meeting_id for both), partitions, and retention.
const (
	TopicMeetingUploaded      = "meeting.uploaded.v1"
	TopicMeetingStatusChanged = "meeting.status-changed.v1"
)

type Publisher struct {
	uploaded      *kafkago.Writer
	statusChanged *kafkago.Writer
}

func NewPublisher(brokers []string) *Publisher {
	return &Publisher{
		uploaded:      kafkax.NewWriter(brokers, TopicMeetingUploaded),
		statusChanged: kafkax.NewWriter(brokers, TopicMeetingStatusChanged),
	}
}

func (p *Publisher) Close() error {
	if err := p.uploaded.Close(); err != nil {
		return err
	}
	return p.statusChanged.Close()
}

// Every Publish* call below keys by meeting_id, per kafka-topics.md's
// "per-meeting ordering matters" rule.

func (p *Publisher) PublishMeetingUploaded(ctx context.Context, event entity.MeetingUploadedEvent) error {
	return kafkax.Publish(ctx, p.uploaded, event.MeetingID, event)
}

func (p *Publisher) PublishMeetingStatusChanged(ctx context.Context, event entity.MeetingStatusChangedEvent) error {
	return kafkax.Publish(ctx, p.statusChanged, event.MeetingID, event)
}

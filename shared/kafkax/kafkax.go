// Package kafkax is the shared Kafka client wrapper — thin factories over
// segmentio/kafka-go plus a JSON-publish helper, the Kafka equivalent of
// shared/redisx and shared/dbx. See
// docs/architecture/kafka-topics.md for topic naming, partitioning, and
// delivery-semantics conventions every producer/consumer follows.
package kafkax

import (
	"context"
	"encoding/json"

	"github.com/segmentio/kafka-go"
)

// NewWriter returns a Writer for topic, keyed writes routed by Hash so
// the same key (meeting_id, per kafka-topics.md's convention) always
// lands on the same partition — required for the per-meeting event
// ordering that doc's "Ordering" section describes. Topics are created
// ahead of time by deployments/kafka-init (see kafka-topics.md's "Local
// dev infra" section), so auto-creation is deliberately left off: a
// publish to a topic that doesn't exist yet is a configuration bug worth
// surfacing, not silently fixing.
func NewWriter(brokers []string, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: false,
	}
}

// NewReader returns a Reader consuming topic under groupID — one group
// per service (see kafka-topics.md's naming convention), so each
// service's replay/rebalance is independent of every other consumer.
// StartOffset is FirstOffset: a brand-new consumer group (this service's
// first-ever boot) should process the backlog rather than silently
// skip to "whatever's produced from now on".
func NewReader(brokers []string, topic, groupID string) *kafka.Reader {
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		GroupID:     groupID,
		StartOffset: kafka.FirstOffset,
	})
}

// Publish JSON-marshals v and writes it to w under key — every producer
// in this system uses plain JSON payloads, no schema registry, per
// kafka-topics.md's own stated convention.
func Publish(ctx context.Context, w *kafka.Writer, key string, v any) error {
	value, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: value})
}

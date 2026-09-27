// Package kafkax is the shared Kafka client wrapper — thin factories over
// segmentio/kafka-go plus a JSON-publish helper, the Kafka equivalent of
// shared/redisx and shared/dbx. See
// docs/architecture/kafka-topics.md for topic naming, partitioning, and
// delivery-semantics conventions every producer/consumer follows.
package kafkax

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
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
// kafka-topics.md's own stated convention. If ctx carries a traceparent
// (see reqctx.WithTraceparent), it's attached as a Kafka message header
// automatically — every producer in this repo goes through Publish, so
// this is the one place trace propagation needs to be wired, not
// something each PublishX method has to remember to do itself.
func Publish(ctx context.Context, w *kafka.Writer, key string, v any) error {
	value, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return w.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: value, Headers: traceHeaders(ctx)})
}

// HeaderTraceparent is the Kafka message header key carrying a W3C
// traceparent value ("00-{32 hex trace-id}-{16 hex parent-id}-{flags}")
// — see docs/architecture/kafka-topics.md's "Trace propagation" rule.
// Phase 2.7 hand-generates and parses this format directly rather than
// pulling in the full OTel SDK (that's Phase 6's job, once a real
// Collector/Tempo backend exists to send spans to — there's nothing to
// visualize a trace in yet, only correlated log lines). The wire format
// is the same either way, so adopting real OTel later needs no
// producer/consumer changes here, just a real span recorded behind the
// same header.
const HeaderTraceparent = "traceparent"

// NewTraceparent starts a new trace — called at the one place each
// Kafka-driven pipeline actually begins (meeting-service's
// ConfirmUploadUseCase, publishing meeting.uploaded.v1). Every event
// downstream of it carries a ChildTraceparent of this same trace-id.
func NewTraceparent() string {
	return "00-" + randomHex(16) + "-" + randomHex(8) + "-01"
}

// ChildTraceparent keeps parent's trace-id but mints a fresh span-id —
// called by every consumer (see each service's consumer.go) before it
// does its own work, so a grep for one trace-id across every service's
// logs shows one meeting's entire pipeline run in order, even without a
// real span tree to view it in yet. An unparseable/missing parent starts
// a fresh trace instead of propagating garbage — a malformed or absent
// tracing header (e.g. a message published before this feature existed)
// should never fail the pipeline.
func ChildTraceparent(parent string) string {
	traceID := TraceIDOf(parent)
	if traceID == "" {
		return NewTraceparent()
	}
	return "00-" + traceID + "-" + randomHex(8) + "-01"
}

// TraceIDOf extracts just the 32-hex-char trace-id segment from a
// traceparent string, for compact log fields (trace_id=... rather than
// the full traceparent=00-...-...-01) — "" for anything that doesn't
// parse as a well-formed traceparent.
func TraceIDOf(traceparent string) string {
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 || len(parts[1]) != 32 {
		return ""
	}
	return parts[1]
}

// TraceparentFromHeaders extracts the traceparent header from a fetched
// message's headers, or "" if none is present. Consumers pass this
// straight into ChildTraceparent, whose own fallback treats "no
// traceparent" as "start a fresh trace here," not an error.
func TraceparentFromHeaders(headers []kafka.Header) string {
	for _, h := range headers {
		if h.Key == HeaderTraceparent {
			return string(h.Value)
		}
	}
	return ""
}

func traceHeaders(ctx context.Context) []kafka.Header {
	tp := reqctx.Traceparent(ctx)
	if tp == "" {
		return nil
	}
	return []kafka.Header{{Key: HeaderTraceparent, Value: []byte(tp)}}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

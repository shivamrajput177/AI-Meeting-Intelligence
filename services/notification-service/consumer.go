// consumer.go is Notification Service's Kafka consumer — one reader per
// topic, both under this service's own "notification-service" consumer
// group so its offsets/replay never affect any other service's consumers.
//
// Like analytics-service/consumer.go, there's no further topic to
// escalate a failure to here: enqueueing an outbox row is this service's
// own "durable write" step (see entity.OutboxRow's doc comment on the
// outbox pattern), not a pipeline stage another service is waiting on. So
// each loop below retries a fixed number of times, then logs and commits
// anyway — a redelivered message that keeps failing (e.g. Meeting
// Service is down) would otherwise block its whole partition forever.
package main

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// Topic names mirror each producing service's own events/kafka package —
// duplicated, not imported, matching every other cross-service constant
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
const (
	topicSummaryCompleted    = "summary.completed.v1"
	topicActionItemExtracted = "action-item.extracted.v1"
)

const maxConsumeAttempts = 3

// consumeGroupID is this service's Kafka consumer group.
const consumeGroupID = "notification-service"

// fetchAndDecode fetches the next message on reader and decodes it into
// out, returning the message and its derived per-message context
// (carrying the trace this message continues, per shared/kafkax's doc
// comment on HeaderTraceparent). decoded is false only for a
// permanently-malformed message (already logged and committed here); the
// caller should just continue its loop.
func fetchAndDecode(ctx context.Context, reader *kafkago.Reader, out any, log *logger.Logger) (msg kafkago.Message, msgCtx context.Context, decoded bool, fetchErr error) {
	topic := reader.Config().Topic
	msg, err := reader.FetchMessage(ctx)
	if err != nil {
		return msg, nil, false, err
	}

	if err := json.Unmarshal(msg.Value, out); err != nil {
		log.Error("decode message", "topic", topic, "err", err)
		_ = reader.CommitMessages(ctx, msg) // will never parse on retry either — commit and move on
		return msg, nil, false, nil
	}

	traceparent := kafkax.ChildTraceparent(kafkax.TraceparentFromHeaders(msg.Headers))
	return msg, reqctx.WithTraceparent(ctx, traceparent), true, nil
}

// retryThenCommit runs process up to maxConsumeAttempts times (with a
// short backoff between attempts), logs on final failure, and always
// commits the offset — see this file's own doc comment for why there's no
// separate failure signal to escalate to instead.
func retryThenCommit(ctx context.Context, reader *kafkago.Reader, msg kafkago.Message, topic string, log *logger.Logger, process func() error) {
	var err error
	for attempt := 1; attempt <= maxConsumeAttempts; attempt++ {
		if err = process(); err == nil {
			break
		}
		log.Error("process message", "topic", topic, "attempt", attempt, "err", err)
		if attempt < maxConsumeAttempts {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
	}
	if err != nil {
		log.Error("giving up on message, committing anyway", "topic", topic, "err", err)
	}
	if err := reader.CommitMessages(ctx, msg); err != nil {
		log.Error("commit offset", "topic", topic, "err", err)
	}
}

func ConsumeSummaryCompleted(ctx context.Context, reader *kafkago.Reader, uc *usecase.EnqueueSummaryNotificationsUseCase, log *logger.Logger) {
	for {
		var event entity.SummaryCompletedEvent
		msg, msgCtx, decoded, err := fetchAndDecode(ctx, reader, &event, log)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch message", "topic", topicSummaryCompleted, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if !decoded {
			continue
		}
		retryThenCommit(ctx, reader, msg, topicSummaryCompleted, log, func() error {
			return uc.EnqueueSummaryNotifications(msgCtx, event.OrgID, event.MeetingID)
		})
	}
}

func ConsumeActionItemExtracted(ctx context.Context, reader *kafkago.Reader, uc *usecase.EnqueueActionItemDigestUseCase, log *logger.Logger) {
	for {
		var event entity.ActionItemExtractedEvent
		msg, msgCtx, decoded, err := fetchAndDecode(ctx, reader, &event, log)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch message", "topic", topicActionItemExtracted, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if !decoded {
			continue
		}
		retryThenCommit(ctx, reader, msg, topicActionItemExtracted, log, func() error {
			return uc.EnqueueActionItemDigest(msgCtx, event.OrgID, event.MeetingID, event.ItemCount)
		})
	}
}

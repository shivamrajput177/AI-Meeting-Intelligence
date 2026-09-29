// consumer.go is Analytics Service's Kafka consumer — the whole service is
// a pure consumer (see docs/architecture/microservices.md §11's "Scaling"
// note), one reader per topic, all under this service's own
// "analytics-service" consumer group so its offsets/replay never affect
// any other service's consumers.
//
// Unlike action-item-service/consumer.go's retry-then-publish-failure
// policy, there's no further topic to escalate to here: Analytics Service
// is the end of the line for every event it reads, nothing downstream
// consumes an analytics failure signal. So each loop below retries a
// fixed number of times, then logs and commits anyway — a redelivered
// message that keeps failing would otherwise block its whole partition
// forever, and these rollups are explicitly disposable/replayable (see
// entity package's doc comment), not a pipeline stage another service is
// waiting on.
package main

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// Topic names mirror each producing service's own events/kafka package —
// duplicated, not imported, matching every other cross-service constant
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
const (
	topicMeetingStatusChanged    = "meeting.status-changed.v1"
	topicActionItemExtracted     = "action-item.extracted.v1"
	topicActionItemStatusChanged = "action-item.status-changed.v1"
	topicSummaryCompleted        = "summary.completed.v1"
)

const maxAttempts = 3

// consumeGroupID is this service's Kafka consumer group — see
// docs/architecture/microservices.md §11: "own consumer group
// analytics-service, offsets independent of every other consumer so
// replay/backfill never affects operational services."
const consumeGroupID = "analytics-service"

// fetchAndDecode fetches the next message on reader and decodes it into
// out, returning the message, its derived per-message context (carrying
// the trace this message continues, per shared/kafkax's doc comment on
// HeaderTraceparent), and its broker timestamp — every rollup usecase
// buckets by that timestamp, not time.Now(), for replay-determinism.
// decoded is false only for a permanently-malformed message (already
// logged and committed here); the caller should just continue its loop.
func fetchAndDecode(ctx context.Context, reader *kafkago.Reader, out any, log *logger.Logger) (msg kafkago.Message, msgCtx context.Context, eventTime time.Time, decoded bool, fetchErr error) {
	topic := reader.Config().Topic
	msg, err := reader.FetchMessage(ctx)
	if err != nil {
		return msg, nil, time.Time{}, false, err
	}

	if err := json.Unmarshal(msg.Value, out); err != nil {
		log.Error("decode message", "topic", topic, "err", err)
		_ = reader.CommitMessages(ctx, msg) // will never parse on retry either — commit and move on
		return msg, nil, time.Time{}, false, nil
	}

	traceparent := kafkax.ChildTraceparent(kafkax.TraceparentFromHeaders(msg.Headers))
	return msg, reqctx.WithTraceparent(ctx, traceparent), msg.Time, true, nil
}

// retryThenCommit runs process up to maxAttempts times (with a short
// backoff between attempts), logs on final failure, and always commits
// the offset — see this file's own doc comment for why there's no
// separate failure signal to escalate to instead.
func retryThenCommit(ctx context.Context, reader *kafkago.Reader, msg kafkago.Message, topic string, log *logger.Logger, process func() error) {
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err = process(); err == nil {
			break
		}
		log.Error("process message", "topic", topic, "attempt", attempt, "err", err)
		if attempt < maxAttempts {
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

func ConsumeMeetingStatusChanged(ctx context.Context, reader *kafkago.Reader, uc *usecase.RecordMeetingStatusChangedUseCase, log *logger.Logger) {
	for {
		var event entity.MeetingStatusChangedEvent
		msg, msgCtx, eventTime, decoded, err := fetchAndDecode(ctx, reader, &event, log)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch message", "topic", topicMeetingStatusChanged, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if !decoded {
			continue
		}
		retryThenCommit(ctx, reader, msg, topicMeetingStatusChanged, log, func() error {
			return uc.RecordMeetingStatusChanged(msgCtx, event.OrgID, event.MeetingID, event.Status, eventTime)
		})
	}
}

func ConsumeActionItemExtracted(ctx context.Context, reader *kafkago.Reader, uc *usecase.RecordActionItemsExtractedUseCase, log *logger.Logger) {
	for {
		var event entity.ActionItemExtractedEvent
		msg, msgCtx, eventTime, decoded, err := fetchAndDecode(ctx, reader, &event, log)
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
			return uc.RecordActionItemsExtracted(msgCtx, event.OrgID, event.MeetingID, eventTime)
		})
	}
}

func ConsumeActionItemStatusChanged(ctx context.Context, reader *kafkago.Reader, uc *usecase.RecordActionItemStatusChangedUseCase, log *logger.Logger) {
	for {
		var event entity.ActionItemStatusChangedEvent
		msg, msgCtx, eventTime, decoded, err := fetchAndDecode(ctx, reader, &event, log)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch message", "topic", topicActionItemStatusChanged, "err", err)
			time.Sleep(time.Second)
			continue
		}
		if !decoded {
			continue
		}
		retryThenCommit(ctx, reader, msg, topicActionItemStatusChanged, log, func() error {
			return uc.RecordActionItemStatusChanged(msgCtx, event.OrgID, event.OwnerUserID, event.Status, eventTime)
		})
	}
}

func ConsumeSummaryCompleted(ctx context.Context, reader *kafkago.Reader, uc *usecase.RecordTopicsFromSummaryUseCase, log *logger.Logger) {
	for {
		var event entity.SummaryCompletedEvent
		msg, msgCtx, eventTime, decoded, err := fetchAndDecode(ctx, reader, &event, log)
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
			return uc.RecordTopicsFromSummary(msgCtx, event.OrgID, event.MeetingID, eventTime)
		})
	}
}

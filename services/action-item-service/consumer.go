// consumer.go is Action Item Service's Kafka consumer loop — the Kafka
// analogue of routes.go ("which topic maps to which usecase"), plus the
// retry/failure-signal policy docs/architecture/kafka-topics.md's
// "Delivery Semantics & Reliability" section documents: exponential
// backoff for a fixed number of attempts, then give up and publish the
// topic's own *.failed.v1-equivalent signal rather than blocking the
// partition forever — same shape as aisummarysvc/consumer.go.
package main

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// topicSummaryCompleted mirrors aisummarysvc/events/kafka's
// TopicSummaryCompleted — duplicated, not imported, matching every other
// cross-service constant in this repo (services never import each
// other's Go packages).
const topicSummaryCompleted = "summary.completed.v1"

const maxAttempts = 3

// ConsumeSummaryCompleted runs until ctx is cancelled, fetching each
// summary.completed.v1 message in turn and committing its offset only
// after it's been handled — successfully, or by publishing
// action-item.extraction-failed.v1 once retries are exhausted — never on
// a still-retryable error, so a process crash mid-retry picks the same
// message back up on restart instead of silently losing it.
func ConsumeSummaryCompleted(ctx context.Context, reader *kafkago.Reader, extract *usecase.ExtractActionItemsUseCase, publisher events.Publisher, log *logger.Logger) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch summary.completed.v1", "err", err)
			time.Sleep(time.Second)
			continue
		}

		var event entity.SummaryCompletedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Error("decode summary.completed.v1", "err", err)
			_ = reader.CommitMessages(ctx, msg) // will never parse on retry either — commit and move on
			continue
		}

		var procErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if _, procErr = extract.ExtractActionItems(ctx, event.OrgID, event.MeetingID); procErr == nil {
				break
			}
			log.Error("process summary.completed.v1", "meeting_id", event.MeetingID, "attempt", attempt, "err", procErr)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
		}
		if procErr != nil {
			if err := publisher.PublishActionItemExtractionFailed(ctx, entity.ActionItemExtractionFailedEvent{
				MeetingID: event.MeetingID, OrgID: event.OrgID, Reason: procErr.Error(),
			}); err != nil {
				log.Error("publish action-item.extraction-failed.v1", "meeting_id", event.MeetingID, "err", err)
			}
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Error("commit summary.completed.v1 offset", "meeting_id", event.MeetingID, "err", err)
		}
	}
}

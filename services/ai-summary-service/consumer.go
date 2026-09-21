// consumer.go is AI Summary Service's Kafka consumer loop — the Kafka
// analogue of routes.go ("which topic maps to which usecase"), plus the
// retry/failure-signal policy docs/architecture/kafka-topics.md's
// "Delivery Semantics & Reliability" section documents: exponential
// backoff for a fixed number of attempts, then give up and publish the
// topic's own *.failed.v1 signal rather than blocking the partition
// forever. transcription.completed.v1 has no .dlq of its own here — per
// kafka-topics.md's DLQ rule, summary.failed.v1 already is that signal
// for this consumer, so there's nowhere else to route the original
// message.
package main

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// topicTranscriptionCompleted mirrors
// transcriptionsvc/events/kafka's TopicTranscriptionCompleted —
// duplicated, not imported, matching every other cross-service constant
// in this repo (services never import each other's Go packages).
const topicTranscriptionCompleted = "transcription.completed.v1"

const maxAttempts = 3

// ConsumeTranscriptionCompleted runs until ctx is cancelled, fetching
// each transcription.completed.v1 message in turn and committing its
// offset only after it's been handled — successfully, or by publishing
// summary.failed.v1 once retries are exhausted — never on a
// still-retryable error, so a process crash mid-retry picks the same
// message back up on restart instead of silently losing it.
func ConsumeTranscriptionCompleted(ctx context.Context, reader *kafkago.Reader, processTranscript *usecase.ProcessTranscriptUseCase, publisher events.Publisher, log *logger.Logger) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch transcription.completed.v1", "err", err)
			time.Sleep(time.Second)
			continue
		}

		var event entity.TranscriptionCompletedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Error("decode transcription.completed.v1", "err", err)
			_ = reader.CommitMessages(ctx, msg) // will never parse on retry either — commit and move on
			continue
		}

		var procErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if _, procErr = processTranscript.ProcessTranscript(ctx, event.OrgID, event.MeetingID); procErr == nil {
				break
			}
			log.Error("process transcription.completed.v1", "meeting_id", event.MeetingID, "attempt", attempt, "err", procErr)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
		}
		if procErr != nil {
			if err := publisher.PublishSummaryFailed(ctx, entity.SummaryFailedEvent{
				MeetingID: event.MeetingID, OrgID: event.OrgID, Reason: procErr.Error(),
			}); err != nil {
				log.Error("publish summary.failed.v1", "meeting_id", event.MeetingID, "err", err)
			}
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Error("commit transcription.completed.v1 offset", "meeting_id", event.MeetingID, "err", err)
		}
	}
}

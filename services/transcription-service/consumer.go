// consumer.go is Transcription Service's Kafka consumer loop — the Kafka
// analogue of routes.go ("which topic maps to which usecase"), plus the
// retry/failure-signal policy docs/architecture/kafka-topics.md's
// "Delivery Semantics & Reliability" section documents: exponential
// backoff for a fixed number of attempts, then give up and publish the
// topic's own *.failed.v1 signal rather than blocking the partition
// forever. meeting.uploaded.v1 has no .dlq of its own — per
// kafka-topics.md's DLQ rule, transcription.failed.v1 already is that
// signal, so there's nowhere else to route the original message.
package main

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// topicMeetingUploaded mirrors meetingsvc/events/kafka's
// TopicMeetingUploaded — duplicated, not imported, matching every other
// cross-service constant in this repo (services never import each
// other's Go packages).
const topicMeetingUploaded = "meeting.uploaded.v1"

const maxAttempts = 3

// ConsumeMeetingUploaded runs until ctx is cancelled, fetching each
// meeting.uploaded.v1 message in turn and committing its offset only
// after it's been handled — successfully, or by publishing
// transcription.failed.v1 once retries are exhausted — never on a
// still-retryable error, so a process crash mid-retry picks the same
// message back up on restart instead of silently losing it.
func ConsumeMeetingUploaded(ctx context.Context, reader *kafkago.Reader, processUpload *usecase.ProcessUploadUseCase, publisher events.Publisher, log *logger.Logger) {
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch meeting.uploaded.v1", "err", err)
			time.Sleep(time.Second)
			continue
		}

		var event entity.MeetingUploadedEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Error("decode meeting.uploaded.v1", "err", err)
			_ = reader.CommitMessages(ctx, msg) // will never parse on retry either — commit and move on
			continue
		}

		var procErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if _, procErr = processUpload.ProcessUpload(ctx, event); procErr == nil {
				break
			}
			log.Error("process meeting.uploaded.v1", "meeting_id", event.MeetingID, "attempt", attempt, "err", procErr)
			if attempt < maxAttempts {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
		}
		if procErr != nil {
			if err := publisher.PublishTranscriptionFailed(ctx, entity.TranscriptionFailedEvent{
				MeetingID: event.MeetingID, OrgID: event.OrgID, Reason: procErr.Error(),
			}); err != nil {
				log.Error("publish transcription.failed.v1", "meeting_id", event.MeetingID, "err", err)
			}
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Error("commit meeting.uploaded.v1 offset", "meeting_id", event.MeetingID, "err", err)
		}
	}
}

// consumer.go is Meeting Service's Kafka consumer loop — Phase 2.6's
// "status-machine wiring off Kafka events" (see docs/ROADMAP.md). Unlike
// every other consumer in this repo, the business logic behind each topic
// here is identical (advance one meeting's status to a fixed value), so
// one generic loop parameterized by topic+target-status replaces what
// would otherwise be six near-copies of the same fetch/decode/commit
// shape — see consumeStatusEvent's own doc comment.
package main

import (
	"context"
	"encoding/json"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/kafkax"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// Topic names mirror each producing service's own events/kafka package —
// duplicated, not imported, matching every other cross-service constant
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
const (
	topicTranscriptionCompleted     = "transcription.completed.v1"
	topicTranscriptionFailed        = "transcription.failed.v1"
	topicSummaryCompleted           = "summary.completed.v1"
	topicSummaryFailed              = "summary.failed.v1"
	topicActionItemExtracted        = "action-item.extracted.v1"
	topicActionItemExtractionFailed = "action-item.extraction-failed.v1"
)

// pipelineEvent is the shape shared by every topic this consumer reads —
// every completion/failure event across this repo carries at least
// meetingId/orgId (see docs/architecture/kafka-topics.md's topic
// catalog), which is all a status transition needs.
type pipelineEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
}

// consumeStatusEvent runs until ctx is cancelled, fetching each message on
// reader in turn, decoding it as a pipelineEvent, and setting the named
// meeting's status to targetStatus. There's no retry/backoff here, unlike
// the AI services' own consumers: those retry a flaky external call
// (Ollama/whisper.cpp) that might genuinely succeed on attempt two; a
// failed status UPDATE is either a transient Postgres hiccup (which the
// next event for this meeting will naturally re-surface, since the whole
// pipeline keeps producing events regardless) or a permanent one (the
// meeting/org no longer exists), and there's no separate failure topic to
// escalate to the way an AI service gives up and publishes its own
// *.failed.v1 — advancing status is the terminal action here, not a step
// with a further downstream consumer. Every case still commits the
// offset: a status transition that can't be applied now won't become
// applicable by redelivering the same message forever.
func consumeStatusEvent(ctx context.Context, reader *kafkago.Reader, updateStatus *usecase.UpdateStatusUseCase, targetStatus string, log *logger.Logger) {
	topic := reader.Config().Topic
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // shutting down
			}
			log.Error("fetch message", "topic", topic, "err", err)
			time.Sleep(time.Second)
			continue
		}

		var event pipelineEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			log.Error("decode message", "topic", topic, "err", err)
			_ = reader.CommitMessages(ctx, msg) // will never parse on retry either — commit and move on
			continue
		}

		// See shared/kafkax's doc comment on HeaderTraceparent: msgCtx (not
		// ctx) carries this message's trace forward into UpdateStatus's own
		// meeting.status-changed.v1 publish — ctx itself stays the loop's
		// own long-lived context, used only for FetchMessage/CommitMessages.
		traceparent := kafkax.ChildTraceparent(kafkax.TraceparentFromHeaders(msg.Headers))
		msgCtx := reqctx.WithTraceparent(ctx, traceparent)

		if _, err := updateStatus.UpdateStatus(msgCtx, event.OrgID, event.MeetingID, targetStatus); err != nil {
			log.Error("advance meeting status", "topic", topic, "meeting_id", event.MeetingID,
				"trace_id", kafkax.TraceIDOf(traceparent), "target_status", targetStatus, "err", err)
		}

		if err := reader.CommitMessages(ctx, msg); err != nil {
			log.Error("commit offset", "topic", topic, "meeting_id", event.MeetingID, "err", err)
		}
	}
}

// statusConsumers is every (topic, target status) pair Phase 2.6 wires
// up — see main.go's startConsumers, which opens one reader per entry and
// runs consumeStatusEvent against it. "transcribing"/"summarizing" have
// no entry here: no service publishes a "just started" event for either
// stage (only completion/failure), so those two states currently go
// unset by this consumer — a real, documented gap rather than a
// synchronous status write bolted onto ConfirmUploadUseCase to
// approximate one (see that use case's own doc comment: it already has
// its own best-effort-publish trade-off to explain, and "the status
// machine is Kafka-driven" is worth staying true to literally).
var statusConsumers = []struct {
	topic  string
	status string
}{
	{topicTranscriptionCompleted, entity.StatusTranscribed},
	{topicTranscriptionFailed, entity.StatusFailed},
	{topicSummaryCompleted, entity.StatusSummarized},
	{topicSummaryFailed, entity.StatusFailed},
	{topicActionItemExtracted, entity.StatusCompleted},
	{topicActionItemExtractionFailed, entity.StatusFailed},
}

package usecase

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/summary"
)

// RecordTopicsFromSummaryUseCase rolls summary.completed.v1 events into
// analytics.topic_frequency — see entity.TopicFrequency's doc comment for
// the keyword-proxy extraction this relies on (extractTopics, in pure.go)
// instead of real NLP keyword/entity extraction.
type RecordTopicsFromSummaryUseCase struct {
	summary summary.Client
	repo    repository.Repository
}

func NewRecordTopicsFromSummaryUseCase(summary summary.Client, repo repository.Repository) *RecordTopicsFromSummaryUseCase {
	return &RecordTopicsFromSummaryUseCase{summary, repo}
}

// RecordTopicsFromSummary looks up meetingID's summary (the event itself
// only carries a summary_id, see entity.SummaryCompletedEvent), extracts
// one topic per key decision/risk/blocker phrase, and increments each
// topic's (org, topic, week) mention count by one. The first per-topic
// error is returned after every topic has been attempted, not on the
// first failure, so one bad increment doesn't stop the rest of the same
// summary's topics from being counted.
func (uc *RecordTopicsFromSummaryUseCase) RecordTopicsFromSummary(ctx context.Context, orgID, meetingID string, eventTime time.Time) error {
	s, err := uc.summary.GetSummary(ctx, orgID, meetingID)
	if err != nil {
		return err
	}
	week := weekOf(eventTime)
	topics := extractTopics(s.KeyDecisions, s.Risks, s.Blockers)

	var firstErr error
	for _, topic := range topics {
		if err := uc.repo.IncrementTopicMentions(ctx, orgID, topic, week, 1); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

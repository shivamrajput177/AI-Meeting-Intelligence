package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/actionitems"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/summary"
)

// fakeRepository, fakeMeetingsClient, fakeActionItemsClient, and
// fakeSummaryClient are in-memory stand-ins for the real
// Postgres-/HTTP-backed implementations — this package's own tests never
// touch a live Postgres or another service over HTTP.

type incrementCall struct {
	orgID, ownerUserID string
	day                time.Time
}

type topicCall struct {
	orgID, topic string
	week         time.Time
	count        int
}

type fakeRepository struct {
	mu sync.Mutex

	meetingCompletions []struct {
		orgID           string
		day             time.Time
		durationMinutes int
	}
	opened []incrementCall
	closed []incrementCall
	topics []topicCall

	trends         []entity.MeetingDailyRollup
	productivity   []entity.ActionItemRollup
	completionOpen int
	completionDone int
	topTopics      []entity.TopicFrequency

	err error
}

func (f *fakeRepository) UpsertMeetingCompletion(_ context.Context, orgID string, day time.Time, durationMinutes int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.meetingCompletions = append(f.meetingCompletions, struct {
		orgID           string
		day             time.Time
		durationMinutes int
	}{orgID, day, durationMinutes})
	return nil
}

func (f *fakeRepository) IncrementActionItemOpened(_ context.Context, orgID, ownerUserID string, day time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.opened = append(f.opened, incrementCall{orgID, ownerUserID, day})
	return nil
}

func (f *fakeRepository) IncrementActionItemClosed(_ context.Context, orgID, ownerUserID string, day time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.closed = append(f.closed, incrementCall{orgID, ownerUserID, day})
	return nil
}

func (f *fakeRepository) IncrementTopicMentions(_ context.Context, orgID, topic string, week time.Time, count int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.topics = append(f.topics, topicCall{orgID, topic, week, count})
	return nil
}

func (f *fakeRepository) GetMeetingTrends(context.Context, string, int) ([]entity.MeetingDailyRollup, error) {
	return f.trends, f.err
}

func (f *fakeRepository) GetProductivity(context.Context, string) ([]entity.ActionItemRollup, error) {
	return f.productivity, f.err
}

func (f *fakeRepository) GetCompletionRate(context.Context, string) (int, int, error) {
	return f.completionOpen, f.completionDone, f.err
}

func (f *fakeRepository) GetTopTopics(context.Context, string, int, int) ([]entity.TopicFrequency, error) {
	return f.topTopics, f.err
}

type fakeMeetingsClient struct {
	durationMinutes int
	err             error
}

func (f *fakeMeetingsClient) GetDurationMinutes(context.Context, string, string) (int, error) {
	return f.durationMinutes, f.err
}

type fakeActionItemsClient struct {
	items []actionitems.Item
	err   error
}

func (f *fakeActionItemsClient) ListForMeeting(context.Context, string, string) ([]actionitems.Item, error) {
	return f.items, f.err
}

type fakeSummaryClient struct {
	summary *summary.Summary
	err     error
}

func (f *fakeSummaryClient) GetSummary(context.Context, string, string) (*summary.Summary, error) {
	return f.summary, f.err
}

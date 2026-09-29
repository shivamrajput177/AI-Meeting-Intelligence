package usecase

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/repository"
)

// trendDays/topicWeeks are this service's fixed query windows — the REST
// API these back (docs/architecture/api-spec.md's Analytics table) takes
// no date-range query parameters yet, so every request gets the same
// dashboard-sized window rather than an unbounded "every row on record"
// scan.
const (
	trendDays  = 30
	topicWeeks = 12
	topicLimit = 20
)

type GetMeetingTrendsUseCase struct{ repo repository.Repository }

func NewGetMeetingTrendsUseCase(repo repository.Repository) *GetMeetingTrendsUseCase {
	return &GetMeetingTrendsUseCase{repo}
}

func (uc *GetMeetingTrendsUseCase) GetTrends(ctx context.Context, orgID string) ([]entity.MeetingDailyRollup, error) {
	return uc.repo.GetMeetingTrends(ctx, orgID, trendDays)
}

type GetProductivityUseCase struct{ repo repository.Repository }

func NewGetProductivityUseCase(repo repository.Repository) *GetProductivityUseCase {
	return &GetProductivityUseCase{repo}
}

func (uc *GetProductivityUseCase) GetProductivity(ctx context.Context, orgID string) ([]entity.ActionItemRollup, error) {
	return uc.repo.GetProductivity(ctx, orgID)
}

type GetCompletionRateUseCase struct{ repo repository.Repository }

func NewGetCompletionRateUseCase(repo repository.Repository) *GetCompletionRateUseCase {
	return &GetCompletionRateUseCase{repo}
}

// GetCompletionRate returns 0 (not NaN) when nothing's been opened yet —
// a brand-new org with no action items has a well-defined "no data" rate,
// not a divide-by-zero.
func (uc *GetCompletionRateUseCase) GetCompletionRate(ctx context.Context, orgID string) (opened, closed int, rate float64, err error) {
	opened, closed, err = uc.repo.GetCompletionRate(ctx, orgID)
	if err != nil {
		return 0, 0, 0, err
	}
	if opened == 0 {
		return opened, closed, 0, nil
	}
	return opened, closed, float64(closed) / float64(opened), nil
}

type GetTopicsUseCase struct{ repo repository.Repository }

func NewGetTopicsUseCase(repo repository.Repository) *GetTopicsUseCase {
	return &GetTopicsUseCase{repo}
}

func (uc *GetTopicsUseCase) GetTopics(ctx context.Context, orgID string) ([]entity.TopicFrequency, error) {
	return uc.repo.GetTopTopics(ctx, orgID, topicWeeks, topicLimit)
}

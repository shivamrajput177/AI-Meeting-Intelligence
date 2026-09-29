// Package repository defines the interface usecase depends on;
// repository/postgres, right below this package in the same tree,
// implements it. Keeping the interface here instead of off in some
// unrelated package is just where it belongs — its one real
// implementation lives one directory down. usecase still only ever
// depends on this interface type, never on
// *postgres.RollupRepository directly, which is what lets it be
// unit-tested against an in-memory fake with zero Postgres involved.
package repository

import (
	"context"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
)

type Repository interface {
	// UpsertMeetingCompletion increments analytics.meeting_daily_rollup's
	// (org_id, day) row by one meeting and durationMinutes, inserting a
	// fresh row on the first meeting of that day.
	UpsertMeetingCompletion(ctx context.Context, orgID string, day time.Time, durationMinutes int) error

	// IncrementActionItemOpened/Closed increment
	// analytics.action_item_rollup's (org_id, owner_user_id, day) row,
	// inserting a fresh row the first time that combination is seen.
	IncrementActionItemOpened(ctx context.Context, orgID, ownerUserID string, day time.Time) error
	IncrementActionItemClosed(ctx context.Context, orgID, ownerUserID string, day time.Time) error

	// IncrementTopicMentions increments analytics.topic_frequency's
	// (org_id, topic, week) row by count.
	IncrementTopicMentions(ctx context.Context, orgID, topic string, week time.Time, count int) error

	// GetMeetingTrends returns analytics.meeting_daily_rollup's rows for
	// orgID over the last sinceDays days, ordered oldest first.
	GetMeetingTrends(ctx context.Context, orgID string, sinceDays int) ([]entity.MeetingDailyRollup, error)

	// GetProductivity returns one row per owner, each day's opened/closed
	// summed across every day on record for orgID.
	GetProductivity(ctx context.Context, orgID string) ([]entity.ActionItemRollup, error)

	// GetCompletionRate returns the org-wide opened/closed totals across
	// every day on record.
	GetCompletionRate(ctx context.Context, orgID string) (opened, closed int, err error)

	// GetTopTopics returns orgID's topics from the last sinceWeeks weeks,
	// ordered by total mentions descending, capped at limit.
	GetTopTopics(ctx context.Context, orgID string, sinceWeeks, limit int) ([]entity.TopicFrequency, error)
}

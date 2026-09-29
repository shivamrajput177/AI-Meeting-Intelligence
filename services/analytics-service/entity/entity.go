// Package entity holds Analytics Service's plain data structs: its own
// internal model (the three rollup rows — what repository reads out of
// Postgres and usecase operates on), the JSON wire shapes at this
// service's REST boundary, and the Kafka payloads it consumes (duplicated
// from each producing service's own entity package, never imported — see
// docs/architecture/microservices.md §"Internal Communication"). Nothing
// here has behavior: it's data, not a class.
package entity

import "time"

// MeetingDailyRollup is one row of analytics.meeting_daily_rollup — see
// docs/architecture/microservices.md §11's schema sketch. Day is always
// truncated to UTC midnight: this is an event-sourced read model rebuilt
// from meeting.status-changed.v1, so "day" is the Kafka broker timestamp
// of the event that produced this row (see repository/postgres's upsert),
// not time.Now() — replay-determinism, the same reasoning
// docs/architecture/kafka-topics.md's Phase 2.7 trace-propagation section
// already established for this repo's other event-time bucketing.
type MeetingDailyRollup struct {
	OrgID        string
	Day          time.Time
	MeetingCount int
	TotalMinutes int
}

// ActionItemRollup is one row of analytics.action_item_rollup, scoped to
// one (org, owner, day). OwnerUserID is never empty here — an action item
// extracted with no matched owner (OwnerUserID nil, per
// actionitemsvc/entity.ActionItem) has nothing to attribute an "opened"
// count to under this table's per-owner PK, so it's simply not rolled up.
// That's a real, documented gap in per-owner productivity numbers for
// unassigned items, not a bug: docs/architecture/microservices.md §11's
// schema has no "unassigned" bucket, and inventing a sentinel UUID for one
// would need a schema change this phase doesn't make.
type ActionItemRollup struct {
	OrgID       string
	OwnerUserID string
	Day         time.Time
	Opened      int
	Closed      int
}

// TopicFrequency is one row of analytics.topic_frequency, scoped to one
// (org, topic, week). Week is truncated to the Monday starting that ISO
// week. "Topic" here is a real, if crude, proxy for the keyword extraction
// docs/ROADMAP.md's Phase 3 goal describes: each of a summary's key
// decisions/risks/blockers is folded into its own topic string (lowercased
// and trimmed) rather than run through an actual NLP keyword/entity
// extraction pass — see usecase/record_topics.go's doc comment for the
// full reasoning and what a real implementation would add.
type TopicFrequency struct {
	OrgID    string
	Topic    string
	Week     time.Time
	Mentions int
}

// --- handler.go: this service's own REST API. Every response here is a
// pre-aggregated read straight off a rollup table (CQRS read-side, no
// operational-table joins) per docs/architecture/microservices.md §11. ---

type MeetingTrendPoint struct {
	Day          string `json:"day"`
	MeetingCount int    `json:"meetingCount"`
	TotalMinutes int    `json:"totalMinutes"`
}

type MeetingTrendsResponse struct {
	Data []MeetingTrendPoint `json:"data"`
}

type ProductivityEntry struct {
	OwnerUserID string `json:"ownerUserId"`
	Opened      int    `json:"opened"`
	Closed      int    `json:"closed"`
}

type ProductivityResponse struct {
	Data []ProductivityEntry `json:"data"`
}

// CompletionRateResponse is a single org-wide aggregate, not a
// per-day/per-owner breakdown — docs/architecture/api-spec.md's
// "Completion rate, aging open items" description is one summary number,
// which is what GET /analytics/action-items/completion-rate returns.
// "Aging open items" (items still open past some age threshold) isn't
// computed here: it needs each open item's own age, which a rollup count
// alone can't derive — this service's own REST API is a rollup query, and
// getting per-item age back into it would mean the exact
// join-through-to-operational-tables CQRS is meant to avoid. A real
// implementation would compute aging directly from action-item-service's
// own data instead of this table.
type CompletionRateResponse struct {
	OpenedTotal    int     `json:"openedTotal"`
	ClosedTotal    int     `json:"closedTotal"`
	CompletionRate float64 `json:"completionRate"`
}

type TopicEntry struct {
	Topic    string `json:"topic"`
	Mentions int    `json:"mentions"`
}

type TopicsResponse struct {
	Data []TopicEntry `json:"data"`
}

// --- Kafka payloads consumed (see docs/architecture/kafka-topics.md). All
// four are duplicated from their producing service's own entity package,
// not imported — the same cross-service-boundary rule every other
// service in this repo follows. ---

// MeetingStatusChangedEvent mirrors meetingsvc/entity.MeetingStatusChangedEvent.
type MeetingStatusChangedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	Status    string `json:"status"`
}

// ActionItemExtractedEvent mirrors actionitemsvc/entity.ActionItemExtractedEvent.
type ActionItemExtractedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	ItemCount int    `json:"itemCount"`
}

// ActionItemStatusChangedEvent mirrors actionitemsvc/entity.ActionItemStatusChangedEvent.
type ActionItemStatusChangedEvent struct {
	ActionItemID string  `json:"actionItemId"`
	MeetingID    string  `json:"meetingId"`
	OrgID        string  `json:"orgId"`
	OwnerUserID  *string `json:"ownerUserId,omitempty"`
	Status       string  `json:"status"`
}

// SummaryCompletedEvent mirrors aisummarysvc/entity.SummaryCompletedEvent.
type SummaryCompletedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	SummaryID string `json:"summaryId"`
}

// --- meetings/*.go, actionitems/*.go, summary/*.go: what each internal
// HTTP client returns, and the wire shapes decoded off the network. Every
// Wire* struct only carries the fields this service actually reads, even
// though the real response has more — unread JSON fields are simply
// ignored by encoding/json. ---

type MeetingWireResponse struct {
	DurationSeconds *int `json:"durationSeconds,omitempty"`
}

// ActionItemWireResponse mirrors the fields of
// actionitemsvc/entity.ActionItemResponse this service actually reads off
// GET /internal/meetings/{id}/action-items.
type ActionItemWireResponse struct {
	ID          string  `json:"id"`
	OwnerUserID *string `json:"ownerUserId,omitempty"`
}

// SummaryWireResponse mirrors the fields of
// aisummarysvc/entity.SummaryResponse this service actually reads off
// GET /internal/meetings/{id}/summary.
type SummaryWireResponse struct {
	KeyDecisions []string `json:"keyDecisions"`
	Risks        []string `json:"risks"`
	Blockers     []string `json:"blockers"`
}

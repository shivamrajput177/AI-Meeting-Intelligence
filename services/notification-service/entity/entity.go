// Package entity holds Notification Service's plain data structs: its own
// internal model (OutboxRow — what repository reads out of Postgres and
// usecase operates on), the per-channel payload shapes stored in that
// row's JSONB column, and the Kafka payloads this service consumes
// (duplicated from each producing service's own entity package, never
// imported — see docs/architecture/microservices.md §"Internal
// Communication") and produces. Nothing here has behavior: it's data, not
// a class.
package entity

import "time"

// Channel values — mirrors docs/architecture/microservices.md §10's
// notification.outbox CHECK (channel IN ('slack','email','jira')). 'jira'
// is accepted by the schema from the start (see migrations/0001_init.up.sql's
// own comment) but nothing enqueues or dispatches it yet — that's Phase
// 4.2/4.3's job.
const (
	ChannelSlack = "slack"
	ChannelEmail = "email"
	ChannelJira  = "jira"
)

// Outbox row status values. Not a DB CHECK constraint (see the migration's
// own doc comment on why) — just documented here.
const (
	StatusPending = "pending" // enqueued, not yet claimed by the poller
	StatusSending = "sending" // claimed by one poller tick, dispatch in flight
	StatusSent    = "sent"    // delivered
	StatusFailed  = "failed"  // permanently gave up after MaxAttempts
)

// OutboxRow is this service's own internal model of one
// notification.outbox row — what repository reads/writes and usecase
// operates on. Not JSON-tagged itself (Payload is already raw JSON bytes,
// decoded per-channel by the dispatcher — see usecase/dispatch.go).
type OutboxRow struct {
	ID        string
	OrgID     string
	Channel   string
	Payload   []byte
	Status    string
	Attempts  int
	LastError *string
	CreatedAt time.Time
	SentAt    *time.Time
}

// SlackPayload is a ChannelSlack row's JSONB payload shape.
type SlackPayload struct {
	Text string `json:"text"`
}

// EmailPayload is a ChannelEmail row's JSONB payload shape.
type EmailPayload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// --- Kafka payloads consumed (see docs/architecture/kafka-topics.md). Both
// are duplicated from their producing service's own entity package, not
// imported — the same cross-service-boundary rule every other service in
// this repo follows. ---

// SummaryCompletedEvent mirrors aisummarysvc/entity.SummaryCompletedEvent.
type SummaryCompletedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	SummaryID string `json:"summaryId"`
}

// ActionItemExtractedEvent mirrors actionitemsvc/entity.ActionItemExtractedEvent.
type ActionItemExtractedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	ItemCount int    `json:"itemCount"`
}

// --- Kafka payloads produced (see docs/architecture/kafka-topics.md's
// notification.sent.v1/notification.failed.v1 rows — documented there
// since this service's own Phase 2.5-2.6 predecessors, but never actually
// published until now). ---

// NotificationSentEvent is notification.sent.v1's payload — a delivery
// audit signal for Analytics Service, not a copy of the message content
// itself.
type NotificationSentEvent struct {
	OutboxID string `json:"outboxId"`
	OrgID    string `json:"orgId"`
	Channel  string `json:"channel"`
}

// NotificationFailedEvent is notification.failed.v1's payload — published
// once a row exhausts MaxAttempts, for alerting rather than any consumer
// to react to (see docs/architecture/microservices.md §10: "(alerting
// only)").
type NotificationFailedEvent struct {
	OutboxID string `json:"outboxId"`
	OrgID    string `json:"orgId"`
	Channel  string `json:"channel"`
	Reason   string `json:"reason"`
}

// --- meetings/*.go: what Client.GetMeeting returns, and the wire shape it
// decodes off the network. MeetingWireResponse only carries the fields
// this service actually reads (title, createdBy) even though
// meetingsvc's real response has more — unread JSON fields are simply
// ignored by encoding/json. ---

type MeetingWireResponse struct {
	Title     string `json:"title"`
	CreatedBy string `json:"createdBy"`
}

// --- users/*.go: what Client.GetEmail returns, and the wire shape it
// decodes off the network. ---

type UserWireResponse struct {
	Email string `json:"email"`
}

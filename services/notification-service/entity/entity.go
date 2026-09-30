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
// notification.outbox CHECK (channel IN ('slack','email','jira')). As of
// Phase 4.2, ChannelJira is enqueued by EnqueueJiraTicketUseCase and
// dispatched by DispatchUseCase via a ticketprovider.Provider (mock only
// for now — AtlassianJiraProvider is Phase 4.3's stretch job).
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

// JiraPayload is a ChannelJira row's JSONB payload shape — just enough
// for DispatchUseCase's ticketprovider.Provider.CreateTicket call
// (ActionItemID to link the ticket, Title as the ticket's summary text).
type JiraPayload struct {
	ActionItemID string `json:"actionItemId"`
	Title        string `json:"title"`
}

// DueReminder is this service's own internal model of one due
// actionitem.reminders row (joined with actionitem.action_items for the
// org_id/description/owner a Slack message needs) — see
// repository.ReminderRepository's own doc comment on why this service
// reads that table directly rather than over REST.
type DueReminder struct {
	ID           string
	ActionItemID string
	OrgID        string
	Description  string
	OwnerUserID  *string
	Channel      string
}

// MockJiraIssue is this service's own internal model of one
// notification.mock_jira_issues row — see
// docs/architecture/deployment-demo-strategy.md §3.
type MockJiraIssue struct {
	ID           string
	OrgID        string
	ActionItemID string
	IssueKey     string
	ProjectKey   string
	Title        string
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Mock Jira board status values — the CHECK constraint on
// notification.mock_jira_issues.status.
const (
	MockStatusToDo       = "To Do"
	MockStatusInProgress = "In Progress"
	MockStatusDone       = "Done"
)

var ValidMockStatuses = map[string]bool{MockStatusToDo: true, MockStatusInProgress: true, MockStatusDone: true}

// --- handler.go: the mock Jira board's own REST API
// (deployment-demo-strategy.md §3's API additions table). ---

type MockJiraIssueResponse struct {
	IssueKey   string `json:"issueKey"`
	ProjectKey string `json:"projectKey"`
	Title      string `json:"title"`
	Status     string `json:"status"`
}

type MockJiraBoardResponse struct {
	Data []MockJiraIssueResponse `json:"data"`
}

// TransitionMockIssueRequest is PATCH
// /orgs/{orgId}/mock-jira/issues/{issueKey}'s body — a manual column
// move, per deployment-demo-strategy.md §3's table.
type TransitionMockIssueRequest struct {
	Status string `json:"status"`
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

// ActionItemJiraRequestedEvent mirrors
// actionitemsvc/entity.ActionItemJiraRequestedEvent.
type ActionItemJiraRequestedEvent struct {
	ActionItemID string `json:"actionItemId"`
	MeetingID    string `json:"meetingId"`
	OrgID        string `json:"orgId"`
	Description  string `json:"description"`
}

// ActionItemReminderDueEvent is action-item.reminder-due.v1's payload —
// unlike every other topic this service touches, both producer and
// consumer are this same service (see
// docs/architecture/kafka-topics.md's flow-4 diagram): the reminder
// scheduler (PublishDueRemindersUseCase) publishes it, and this
// service's own dispatch pipeline (EnqueueReminderUseCase) consumes it
// to enqueue a Slack outbox row — the real reason for the round trip
// through Kafka rather than a direct function call is exactly the same
// as everywhere else in this codebase: Kafka is what makes "the
// reminder was found due" durable independent of whether the process
// that found it is still alive to dispatch it.
type ActionItemReminderDueEvent struct {
	ActionItemID string  `json:"actionItemId"`
	OrgID        string  `json:"orgId"`
	Description  string  `json:"description"`
	OwnerUserID  *string `json:"ownerUserId,omitempty"`
	Channel      string  `json:"channel"`
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

// --- actionitems/*.go: the wire shape Client.UpdateActionItem sends.
// Mirrors actionitemsvc/entity.UpdateActionItemInternalRequest exactly
// (duplicated, not imported, same cross-service-boundary reason as the
// Kafka payloads above). ---

type UpdateActionItemInternalRequest struct {
	Status       *string `json:"status,omitempty"`
	JiraIssueKey *string `json:"jiraIssueKey,omitempty"`
}

// --- orgs/*.go: what Client.GetIntegrationConfig returns, and the wire
// shape it decodes off the network. OrgIntegrationConfigWireResponse only
// carries the fields this service actually reads (slackWebhookUrl,
// ticketProvider) even though orgsvc's real response has more (Jira
// fields nothing here uses yet) — unread JSON fields are simply ignored
// by encoding/json. ---

type OrgIntegrationConfigWireResponse struct {
	SlackWebhookURL *string `json:"slackWebhookUrl,omitempty"`
	TicketProvider  string  `json:"ticketProvider"`
}

// --- handler.go: POST /orgs/{orgId}/integrations/test's request/response
// shapes — see docs/architecture/api-spec.md §Integrations ("Fire a test
// Slack/email/Jira call"). Channel is one of the Channel* consts above;
// To is required only when Channel is ChannelEmail, since nothing else on
// this request (or on org.integration_configs) names an email recipient
// to test against. ---

type TestIntegrationRequest struct {
	Channel string  `json:"channel"`
	To      *string `json:"to,omitempty"`
}

type TestIntegrationResponse struct {
	Channel string `json:"channel"`
	OK      bool   `json:"ok"`
}

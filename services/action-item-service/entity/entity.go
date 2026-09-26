// Package entity holds Action Item Service's plain data structs: its own
// internal model (ActionItem — what repository reads out of Postgres and
// usecase operates on), the JSON wire shapes at this service's REST and
// Kafka boundaries, and the plain input/output structs its usecase layer
// passes around internally. Nothing here has behavior (no methods, just
// fields, and json tags where the struct crosses the wire): it's data,
// not a class, which is what keeps it out of handler/, llm/, summary/,
// participants/, events/, and usecase/ — those packages hold the code
// that does something with an entity, this package only describes its
// shape.
package entity

import "time"

// Valid type/status/priority values — see
// docs/architecture/microservices.md §8's actionitem.action_items CHECK
// constraints, mirrored here so an invalid value from the LLM's output or
// a PATCH request is caught before it ever reaches Postgres.
const (
	TypeAction   = "action"
	TypeDecision = "decision"
	TypeRisk     = "risk"
	TypeBlocker  = "blocker"

	StatusOpen       = "open"
	StatusInProgress = "in_progress"
	StatusDone       = "done"
	StatusCancelled  = "cancelled"

	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

var ValidTypes = map[string]bool{TypeAction: true, TypeDecision: true, TypeRisk: true, TypeBlocker: true}
var ValidStatuses = map[string]bool{StatusOpen: true, StatusInProgress: true, StatusDone: true, StatusCancelled: true}
var ValidPriorities = map[string]bool{PriorityLow: true, PriorityMedium: true, PriorityHigh: true}

// ActionItem is this service's own internal model — what repository reads
// out of Postgres and usecase operates on. Not JSON-tagged: it never
// crosses the wire directly, handler.go always reshapes it into an
// ActionItemResponse first.
type ActionItem struct {
	ID                   string
	MeetingID            string
	OrgID                string
	Description          string
	Type                 string
	OwnerUserID          *string
	OwnerRawName         *string
	DueDate              *time.Time
	Status               string
	Priority             string
	JiraIssueKey         *string
	ExtractedFromChunkID *string
	Confidence           *float32
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Participant is this service's own Go-to-Go shape for what
// participants.Client.ListParticipants returns — not JSON-tagged (see
// ParticipantWireResponse for the wire shape the client actually decodes
// off Meeting Service's internal endpoint).
type Participant struct {
	UserID      *string
	Email       string
	DisplayName string
}

// --- handler.go: this service's own REST API ---

type ActionItemResponse struct {
	ID                   string   `json:"id"`
	MeetingID            string   `json:"meetingId"`
	Description          string   `json:"description"`
	Type                 string   `json:"type"`
	OwnerUserID          *string  `json:"ownerUserId,omitempty"`
	OwnerRawName         *string  `json:"ownerRawName,omitempty"`
	DueDate              *string  `json:"dueDate,omitempty"`
	Status               string   `json:"status"`
	Priority             string   `json:"priority"`
	JiraIssueKey         *string  `json:"jiraIssueKey,omitempty"`
	ExtractedFromChunkID *string  `json:"extractedFromChunkId,omitempty"`
	Confidence           *float32 `json:"confidence,omitempty"`
	CreatedAt            string   `json:"createdAt"`
	UpdatedAt            string   `json:"updatedAt"`
}

type ListActionItemsResponse struct {
	Data     []ActionItemResponse `json:"data"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"pageSize"`
	Total    int                  `json:"total"`
}

// ListActionItemsFilter is ListActionItemsUseCase's input — the
// cross-meeting GET /action-items query parameters (owner, status, type,
// dueBefore per docs/architecture/api-spec.md's Action Items table), plus
// pagination.
type ListActionItemsFilter struct {
	OwnerUserID string
	Status      string
	Type        string
	DueBefore   *time.Time
	Page        int
	PageSize    int
}

// UpdateActionItemRequest is PATCH /action-items/{id}'s body — every
// field is a pointer so the caller can change just one of status/owner/due
// date without clobbering the others, the actual meaning of a partial
// PATCH (as opposed to user-service's UpdateMeRequest, which replaces its
// whole small field set at once because every field there is always
// present in a profile-edit form anyway).
type UpdateActionItemRequest struct {
	Status      *string `json:"status,omitempty"`
	OwnerUserID *string `json:"ownerUserId,omitempty"`
	DueDate     *string `json:"dueDate,omitempty"`
}

// --- Kafka payloads (see docs/architecture/kafka-topics.md) ---

// SummaryCompletedEvent mirrors aisummarysvc/entity.SummaryCompletedEvent
// — duplicated rather than imported, matching every other service
// boundary in this repo (services never import each other's Go packages,
// only communicate over REST/Kafka).
type SummaryCompletedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	SummaryID string `json:"summaryId"`
}

// ActionItemExtractedEvent is action-item.extracted.v1's payload — a
// batch signal (count only, like ai-summary-service's ChunkCreatedEvent)
// since no consumer needs the individual item IDs yet: Meeting Service
// only needs to know extraction finished to advance the status machine,
// and Notification/Analytics (Phase 4) can read the items themselves via
// this service's own REST API once they exist.
type ActionItemExtractedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	ItemCount int    `json:"itemCount"`
}

// ActionItemExtractionFailedEvent is this service's failure signal — see
// docs/architecture/kafka-topics.md's action-item.extraction-failed.v1
// row, added alongside this service (transcription.failed.v1 and
// summary.failed.v1 already had the equivalent for their own consumers;
// this topic closes the same gap for Action Item Service).
type ActionItemExtractionFailedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	Reason    string `json:"reason"`
}

// --- summary/*.go: what Client.GetSummary returns, and the wire shape it
// decodes off the network. SummaryWireResponse mirrors
// aisummarysvc/entity.SummaryResponse (duplicated, not imported, same
// cross-service-boundary reason as the Kafka payloads above); Summary is
// this service's own Go-to-Go shape. ---

type SummaryWireResponse struct {
	ID           string   `json:"id"`
	MeetingID    string   `json:"meetingId"`
	SummaryText  string   `json:"summaryText"`
	KeyDecisions []string `json:"keyDecisions"`
	Risks        []string `json:"risks"`
	Blockers     []string `json:"blockers"`
}

type Summary struct {
	MeetingID    string
	SummaryText  string
	KeyDecisions []string
	Risks        []string
	Blockers     []string
}

// --- participants/*.go: what Client.ListParticipants returns, and the
// wire shape it decodes off the network. ParticipantWireResponse mirrors
// meetingsvc/entity.ParticipantResponse (duplicated, not imported, same
// reason as above). ---

type ParticipantWireResponse struct {
	UserID      *string `json:"userId,omitempty"`
	Email       string  `json:"email"`
	DisplayName string  `json:"displayName"`
}

// --- usecase/*.go: input for UpdateActionItemUseCase.Execute. Not a JSON
// wire struct (no json tags) — handler.go parses UpdateActionItemRequest's
// string DueDate into a *time.Time before building one of these, the same
// "decode wire shape, then build the usecase's own typed input" split
// meeting-service's CreateUploadIntentInput already uses. ---

type UpdateActionItemInput struct {
	Status      *string
	OwnerUserID *string
	DueDate     *time.Time
}

// --- llm/*.go: what an Extractor returns. Not a JSON wire struct. ---

// ExtractedItem is one item the LLM extraction prompt returned, before
// owner-matching or persistence — OwnerRawName is whatever name string
// the model produced (possibly empty), not yet resolved against
// participants.
type ExtractedItem struct {
	Description  string
	Type         string
	OwnerRawName string
	DueDate      *time.Time
	Priority     string
	Confidence   float32
}

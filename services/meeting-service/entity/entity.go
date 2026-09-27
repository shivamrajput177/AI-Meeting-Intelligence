// Package entity holds Meeting Service's plain data structs: its own
// internal model (Meeting, ListFilter — what repository reads out of
// Postgres and usecase operates on), the JSON wire shapes at this
// service's REST boundary, and the plain input/output structs its
// usecase layer passes around internally. Nothing here has behavior (no
// methods, just fields, and json tags where the struct crosses the
// wire): it's data, not a class, which is what keeps it out of
// handler/ and usecase/ — those packages hold the code that does
// something with an entity, this package only describes its shape.
package entity

import "time"

// Status values. Since Phase 2.6, consumer.go's statusConsumers drive
// "uploaded" -> "transcribed" -> "summarized" -> "completed" (or
// "failed" from any stage) off transcription/summary/action-item
// completion and failure events — see that file's own doc comment for
// why "transcribing"/"summarizing" specifically are not currently set by
// anything (no service publishes a "just started" event for either
// stage, only completion/failure). PATCH /meetings/{id}/status remains
// available as a manual override on top of that — the Phase 1 "debug
// endpoint" docs/ROADMAP.md describes, still useful for testing/demoing
// without running the real pipeline.
const (
	StatusUploaded     = "uploaded"
	StatusTranscribing = "transcribing"
	StatusTranscribed  = "transcribed"
	StatusSummarizing  = "summarizing"
	StatusSummarized   = "summarized"
	StatusCompleted    = "completed"
	StatusFailed       = "failed"
)

var ValidStatuses = map[string]bool{
	StatusUploaded: true, StatusTranscribing: true, StatusTranscribed: true,
	StatusSummarizing: true, StatusSummarized: true, StatusCompleted: true, StatusFailed: true,
}

// Meeting is this service's own internal model — what repository reads
// out of Postgres and usecase operates on. Not JSON-tagged: it never
// crosses the wire directly, handler.go always reshapes it into a
// MeetingResponse first (see toMeetingResponse).
type Meeting struct {
	ID                 string
	OrgID              string
	Title              string
	CreatedBy          string
	Status             string
	SourceType         string
	RecordingObjectKey string
	DurationSeconds    *int
	StartedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ListFilter is repository.Repository.List's pagination input.
type ListFilter struct {
	Page     int
	PageSize int
}

type MeetingResponse struct {
	ID              string  `json:"id"`
	OrgID           string  `json:"orgId"`
	Title           string  `json:"title"`
	CreatedBy       string  `json:"createdBy"`
	Status          string  `json:"status"`
	SourceType      string  `json:"sourceType"`
	DurationSeconds *int    `json:"durationSeconds,omitempty"`
	StartedAt       *string `json:"startedAt,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type CreateMeetingRequest struct {
	Title string `json:"title"`
}

type CreateMeetingResponse struct {
	MeetingID string `json:"meetingId"`
	UploadURL string `json:"uploadUrl"`
}

type ListMeetingsResponse struct {
	Data     []MeetingResponse `json:"data"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int               `json:"total"`
}

// UpdateStatusRequest is the body of the Phase 1 "debug endpoint" for
// manually flipping a meeting's status — see
// handler.Handler.UpdateStatus's doc comment.
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

// --- usecase/*.go: input/output for
// CreateUploadIntentUseCase.CreateUploadIntent. Not JSON wire structs (no
// json tags) — CreateUploadIntentInput is the usecase layer's own
// Go-to-Go call contract, passed by handler/ straight from a decoded
// CreateMeetingRequest; CreateUploadIntentOutput is what handler/
// reshapes into a CreateMeetingResponse. ---

type CreateUploadIntentInput struct {
	OrgID     string
	CreatedBy string
	Title     string
}

type CreateUploadIntentOutput struct {
	MeetingID string
	UploadURL string
}

// Participant is one row of meeting.participants — this service's own
// internal model, not JSON-tagged (see ParticipantResponse for the wire
// shape Action Item Service's internal client actually decodes).
type Participant struct {
	UserID      *string
	Email       string
	DisplayName string
}

// ParticipantResponse is what GET /internal/meetings/{id}/participants
// returns — see handler.GetParticipantsInternal's doc comment for who
// calls this and why it's internal-only.
type ParticipantResponse struct {
	UserID      *string `json:"userId,omitempty"`
	Email       string  `json:"email"`
	DisplayName string  `json:"displayName"`
}

// MeetingUploadedEvent is meeting.uploaded.v1's payload (see
// docs/architecture/kafka-topics.md) — published once ConfirmUpload
// verifies the recording actually landed in MinIO. Carries everything
// Transcription Service needs to fetch and process the recording without
// a synchronous callback to this service.
type MeetingUploadedEvent struct {
	MeetingID          string `json:"meetingId"`
	OrgID              string `json:"orgId"`
	Title              string `json:"title"`
	SourceType         string `json:"sourceType"`
	RecordingObjectKey string `json:"recordingObjectKey"`
}

// MeetingStatusChangedEvent is meeting.status-changed.v1's payload (see
// docs/architecture/kafka-topics.md) — published by
// UpdateStatusUseCase.UpdateStatus every time it actually changes a
// meeting's status, whether that call came from consumer.go's
// Kafka-driven status machine (Phase 2.6) or the manual PATCH
// /meetings/{id}/status debug endpoint. No consumer exists yet
// (Analytics and Notification, per the topic catalog, are Phase 3/4) —
// published anyway, same precedent as ai-summary-service's
// chunk.created.v1.
type MeetingStatusChangedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	Status    string `json:"status"`
}

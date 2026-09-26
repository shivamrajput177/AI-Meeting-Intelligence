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

// Status values. Phase 1 only ever produces "uploaded" — the rest of the
// pipeline (transcribing -> ... -> completed) doesn't exist until
// Phase 2's Kafka-driven services land. See PATCH /meetings/{id}/status
// for the manual override docs/ROADMAP.md Phase 1 calls "a debug
// endpoint," used to simulate the pipeline advancing before it's real.
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

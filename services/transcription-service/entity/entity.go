// Package entity holds Transcription Service's plain data structs: its
// own internal model (Transcript, Segment — what repository reads out of
// Postgres and usecase operates on), the JSON wire shapes at this
// service's REST and Kafka boundaries, and the plain input/output
// structs its usecase layer passes around internally. Nothing here has
// behavior (no methods, just fields, and json tags where the struct
// crosses the wire): it's data, not a class, which is what keeps it out
// of handler/, asr/, storage/, events/, and usecase/ — those packages
// hold the code that does something with an entity, this package only
// describes its shape.
package entity

import "time"

const (
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Transcript is this service's own internal model — what repository
// reads out of Postgres and usecase operates on. Not JSON-tagged: it
// never crosses the wire directly, handler.go always reshapes it (with
// its Segments) into a TranscriptResponse first.
type Transcript struct {
	ID        string
	MeetingID string
	OrgID     string
	Language  string
	Engine    string
	ModelName string
	Status    string
	RawText   string
	WordCount int
	CreatedAt time.Time
}

// Segment is one time-stamped span of a Transcript. See
// docs/architecture/microservices.md §6's schema for why it has no
// org_id of its own — tenant isolation goes through a join back to
// Transcript, which does carry org_id (see
// migrations/0001_init.up.sql's RLS policy).
type Segment struct {
	ID           int64
	TranscriptID string
	SpeakerLabel string
	StartMS      int
	EndMS        int
	Text         string
	Confidence   float32
}

// --- handler.go: this service's own REST API ---

type SegmentResponse struct {
	SpeakerLabel string  `json:"speakerLabel"`
	StartMs      int     `json:"startMs"`
	EndMs        int     `json:"endMs"`
	Text         string  `json:"text"`
	Confidence   float32 `json:"confidence,omitempty"`
}

type TranscriptResponse struct {
	ID        string            `json:"id"`
	MeetingID string            `json:"meetingId"`
	Language  string            `json:"language"`
	Engine    string            `json:"engine"`
	ModelName string            `json:"modelName"`
	Status    string            `json:"status"`
	RawText   string            `json:"rawText"`
	WordCount int               `json:"wordCount"`
	Segments  []SegmentResponse `json:"segments"`
	CreatedAt string            `json:"createdAt"`
}

// --- Kafka payloads (see docs/architecture/kafka-topics.md) ---

// MeetingUploadedEvent mirrors meetingsvc/entity.MeetingUploadedEvent —
// duplicated rather than imported, matching every other service boundary
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
type MeetingUploadedEvent struct {
	MeetingID          string `json:"meetingId"`
	OrgID              string `json:"orgId"`
	Title              string `json:"title"`
	SourceType         string `json:"sourceType"`
	RecordingObjectKey string `json:"recordingObjectKey"`
}

type TranscriptionCompletedEvent struct {
	MeetingID    string `json:"meetingId"`
	OrgID        string `json:"orgId"`
	TranscriptID string `json:"transcriptId"`
}

// TranscriptionFailedEvent is this service's failure signal — per
// kafka-topics.md's DLQ rule, a *.failed.v1 topic doesn't get its own
// .dlq companion, because it already *is* the "this failed" signal for
// meeting.uploaded.v1 (which has no DLQ of its own for the same reason).
type TranscriptionFailedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	Reason    string `json:"reason"`
}

// --- asr/*.go: what a Transcriber returns, and what handler-facing
// segments get built from. Not JSON wire structs. ---

type TranscriptionResult struct {
	Language string
	Text     string
	Segments []TranscribedSegment
}

type TranscribedSegment struct {
	StartMS int
	EndMS   int
	Text    string
}

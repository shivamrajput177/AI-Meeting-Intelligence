package domain

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

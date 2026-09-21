// Package entity holds AI Summary Service's plain data structs: its own
// internal model (Summary, Chunk — what repository reads out of Postgres
// and usecase operates on), the JSON wire shapes at this service's REST
// and Kafka boundaries, and the plain input/output structs its usecase
// layer passes around internally. Nothing here has behavior (no methods,
// just fields, and json tags where the struct crosses the wire): it's
// data, not a class, which is what keeps it out of handler/, llm/,
// transcript/, events/, and usecase/ — those packages hold the code that
// does something with an entity, this package only describes its shape.
package entity

import "time"

// Summary is this service's own internal model — what repository reads
// out of Postgres and usecase operates on. Not JSON-tagged: it never
// crosses the wire directly, handler.go always reshapes it into a
// SummaryResponse first.
type Summary struct {
	ID            string
	MeetingID     string
	OrgID         string
	SummaryText   string
	KeyDecisions  []string
	Risks         []string
	Blockers      []string
	ModelUsed     string
	PromptVersion string
	CreatedAt     time.Time
}

// Chunk is one ~500-token, speaker-aware window of a meeting's
// transcript — see docs/architecture/microservices.md §7's chunking
// pipeline description. Embeddings for these are stored by Search
// Service (Phase 3), not here — this table is just the LLM-input-writer
// side.
type Chunk struct {
	ID         string
	MeetingID  string
	OrgID      string
	ChunkIndex int
	Text       string
	TokenCount int
	StartMS    int
	EndMS      int
}

// --- handler.go: this service's own REST API ---

type SummaryResponse struct {
	ID            string   `json:"id"`
	MeetingID     string   `json:"meetingId"`
	SummaryText   string   `json:"summaryText"`
	KeyDecisions  []string `json:"keyDecisions"`
	Risks         []string `json:"risks"`
	Blockers      []string `json:"blockers"`
	ModelUsed     string   `json:"modelUsed"`
	PromptVersion string   `json:"promptVersion"`
	CreatedAt     string   `json:"createdAt"`
}

// --- Kafka payloads (see docs/architecture/kafka-topics.md) ---

// TranscriptionCompletedEvent mirrors
// transcriptionsvc/entity.TranscriptionCompletedEvent — duplicated
// rather than imported, matching every other service boundary in this
// repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
type TranscriptionCompletedEvent struct {
	MeetingID    string `json:"meetingId"`
	OrgID        string `json:"orgId"`
	TranscriptID string `json:"transcriptId"`
}

// ChunkCreatedEvent is chunk.created.v1's payload — deliberately just a
// count, not the chunk IDs/text themselves: there's no consumer yet
// (Search Service is Phase 3), and adding fields later is
// additive/backward-compatible per kafka-topics.md's own stated
// convention, so there's nothing to design speculatively here.
type ChunkCreatedEvent struct {
	MeetingID  string `json:"meetingId"`
	OrgID      string `json:"orgId"`
	ChunkCount int    `json:"chunkCount"`
}

type SummaryCompletedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	SummaryID string `json:"summaryId"`
}

// SummaryFailedEvent is this service's failure signal — per
// kafka-topics.md's DLQ rule, a *.failed.v1 topic doesn't get its own
// .dlq companion, because it already *is* the "this failed" signal for
// transcription.completed.v1 (whose own consumers, this service
// included, have no DLQ of their own for the same reason).
type SummaryFailedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	Reason    string `json:"reason"`
}

// --- transcript/*.go: what Client.GetTranscript returns, and the wire
// shape it decodes off the network. TranscriptWireResponse/
// SegmentWireResponse mirror transcriptionsvc/entity.TranscriptResponse/
// SegmentResponse (duplicated, not imported, for the same
// cross-service-boundary reason as the Kafka payloads above);
// Transcript/TranscriptSegment are this service's own Go-to-Go shape,
// not JSON-tagged, that transcript/http's client maps the wire response
// into. ---

type SegmentWireResponse struct {
	SpeakerLabel string  `json:"speakerLabel"`
	StartMs      int     `json:"startMs"`
	EndMs        int     `json:"endMs"`
	Text         string  `json:"text"`
	Confidence   float32 `json:"confidence,omitempty"`
}

type TranscriptWireResponse struct {
	MeetingID string                `json:"meetingId"`
	RawText   string                `json:"rawText"`
	Segments  []SegmentWireResponse `json:"segments"`
}

type TranscriptSegment struct {
	SpeakerLabel string
	StartMS      int
	EndMS        int
	Text         string
}

type Transcript struct {
	MeetingID string
	RawText   string
	Segments  []TranscriptSegment
}

// --- llm/*.go: what a Summarizer returns. Not a JSON wire struct. ---

type SummaryResult struct {
	SummaryText  string
	KeyDecisions []string
	Risks        []string
	Blockers     []string
}

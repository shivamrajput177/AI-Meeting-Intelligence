// Package entity holds Search Service's plain data structs: its own
// internal model (ChunkEmbedding, QAHistoryEntry, SearchHit — what
// repository reads out of Postgres and usecase operates on), the JSON
// wire shapes at this service's REST and Kafka boundaries, and the plain
// input/output structs its usecase layer passes around internally.
// Nothing here has behavior (no methods, just fields, and json tags
// where the struct crosses the wire): it's data, not a class, which is
// what keeps it out of handler/, llm/, chunks/, meetings/, events/, and
// usecase/ — those packages hold the code that does something with an
// entity, this package only describes its shape.
package entity

import "time"

// ChunkEmbedding is this service's own internal model — what repository
// reads out of Postgres and usecase operates on. Not JSON-tagged: it
// never crosses the wire directly.
type ChunkEmbedding struct {
	ChunkID     string
	MeetingID   string
	OrgID       string
	Embedding   []float32
	ModelName   string
	MeetingText string // carried alongside the embedding only in-memory (never persisted) so usecase can build citations without a second DB round-trip
	StartMS     int
	EndMS       int
	CreatedAt   time.Time
}

// SearchHit is one ranked result out of a semantic search — Score is
// cosine distance (pgvector's `<=>` operator: 0 = identical, 2 = opposite),
// lower is better, per docs/architecture/microservices.md §9's RAG flow.
type SearchHit struct {
	ChunkID   string
	MeetingID string
	Text      string
	StartMS   int
	EndMS     int
	Score     float64
}

// SimilarMeeting is one ranked result out of SimilarMeetingsUseCase —
// Score is the best (minimum) cosine distance between any of the two
// meetings' chunks, the same "closest matching chunk stands in for the
// whole meeting" approximation the underlying SQL query itself makes.
type SimilarMeeting struct {
	MeetingID string
	Score     float64
}

// QAHistoryEntry is this service's own internal model for one row of
// search.qa_history.
type QAHistoryEntry struct {
	ID            string
	OrgID         string
	UserID        string
	Question      string
	Answer        string
	CitedChunkIDs []string
	CreatedAt     time.Time
}

// --- handler.go: this service's own REST API ---

type SearchResultResponse struct {
	ChunkID      string  `json:"chunkId"`
	MeetingID    string  `json:"meetingId"`
	MeetingTitle string  `json:"meetingTitle"`
	Text         string  `json:"text"`
	StartMs      int     `json:"startMs"`
	EndMs        int     `json:"endMs"`
	Score        float64 `json:"score"`
}

type SearchResponse struct {
	Query   string                 `json:"query"`
	Results []SearchResultResponse `json:"results"`
}

type SimilarMeetingResponse struct {
	MeetingID    string  `json:"meetingId"`
	MeetingTitle string  `json:"meetingTitle"`
	Score        float64 `json:"score"`
}

type SimilarMeetingsResponse struct {
	Results []SimilarMeetingResponse `json:"results"`
}

type AskRequest struct {
	Question string `json:"question"`
}

type CitationResponse struct {
	MeetingID    string `json:"meetingId"`
	MeetingTitle string `json:"meetingTitle"`
	StartMs      int    `json:"startMs"`
	EndMs        int    `json:"endMs"`
	Snippet      string `json:"snippet"`
}

type AskResponse struct {
	Answer    string             `json:"answer"`
	Citations []CitationResponse `json:"citations"`
}

type QAHistoryEntryResponse struct {
	ID            string   `json:"id"`
	Question      string   `json:"question"`
	Answer        string   `json:"answer"`
	CitedChunkIDs []string `json:"citedChunkIds"`
	CreatedAt     string   `json:"createdAt"`
}

type QAHistoryResponse struct {
	Data []QAHistoryEntryResponse `json:"data"`
}

// ReindexResponse is POST /search/reindex's body — a per-meeting summary
// rather than a bare count, since a full-org backfill (see
// ReindexUseCase's doc comment) is best-effort per meeting: one
// meeting's embedding failure shouldn't hide whether every other meeting
// in the org actually got reindexed.
type ReindexResponse struct {
	MeetingsReindexed int      `json:"meetingsReindexed"`
	MeetingsFailed    int      `json:"meetingsFailed"`
	FailedMeetingIDs  []string `json:"failedMeetingIds,omitempty"`
}

// --- Kafka payloads (see docs/architecture/kafka-topics.md) ---

// ChunkCreatedEvent mirrors aisummarysvc/entity.ChunkCreatedEvent —
// duplicated rather than imported, matching every other service boundary
// in this repo (services never import each other's Go packages, only
// communicate over REST/Kafka).
type ChunkCreatedEvent struct {
	MeetingID  string `json:"meetingId"`
	OrgID      string `json:"orgId"`
	ChunkCount int    `json:"chunkCount"`
}

// EmbeddingCompletedEvent is embedding.completed.v1's payload.
type EmbeddingCompletedEvent struct {
	MeetingID  string `json:"meetingId"`
	OrgID      string `json:"orgId"`
	ChunkCount int    `json:"chunkCount"`
}

// EmbeddingFailedEvent is this service's failure signal — see
// docs/architecture/kafka-topics.md's embedding.failed.v1 row, added
// alongside this service the same way transcription.failed.v1 and
// summary.failed.v1 exist for their own consumers.
type EmbeddingFailedEvent struct {
	MeetingID string `json:"meetingId"`
	OrgID     string `json:"orgId"`
	Reason    string `json:"reason"`
}

// --- chunks/*.go: what Client.ListChunks returns, and the wire shape it
// decodes off the network. ChunkWireResponse mirrors
// aisummarysvc/entity.ChunkResponse (duplicated, not imported, same
// cross-service-boundary reason as the Kafka payloads above); Chunk is
// this service's own Go-to-Go shape. ---

type ChunkWireResponse struct {
	ID         string `json:"id"`
	MeetingID  string `json:"meetingId"`
	ChunkIndex int    `json:"chunkIndex"`
	Text       string `json:"text"`
	TokenCount int    `json:"tokenCount"`
	StartMs    int    `json:"startMs"`
	EndMs      int    `json:"endMs"`
}

type Chunk struct {
	ID        string
	MeetingID string
	Text      string
	StartMS   int
	EndMS     int
}

// --- meetings/*.go: what Client.GetMeeting/ListMeetings return, and the
// wire shape they decode off the network. MeetingWireResponse only
// carries the fields this service actually reads (id, title) even though
// meetingsvc's real response has more — unread JSON fields are simply
// ignored by encoding/json, so there's no need to mirror the full shape
// the way the Kafka payload structs above do (nothing here re-serializes
// this struct onward). ---

type MeetingWireResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type MeetingListWireResponse struct {
	Data     []MeetingWireResponse `json:"data"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Total    int                   `json:"total"`
}

// Meeting is this service's own Go-to-Go shape for a meeting lookup —
// not JSON-tagged.
type Meeting struct {
	ID    string
	Title string
}

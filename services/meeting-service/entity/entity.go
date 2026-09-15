// Package entity holds Meeting Service's plain data structs — the JSON
// wire shapes at this service's REST boundary, and the plain
// input/output structs its usecase layer passes around internally.
// Nothing here has behavior (no methods, just fields, and json tags
// where the struct crosses the wire): it's data, not a class, which is
// what keeps it out of handler/ and usecase/ — those packages hold the
// code that does something with an entity, this package only describes
// its shape.
package entity

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

// Package entity holds Meeting Service's plain request/response structs
// — the JSON wire shapes at this service's REST boundary. Nothing here
// has behavior (no methods, just fields and json tags): it's data, not a
// class, which is what keeps it out of handler/ — that package holds the
// code that does something with an entity, this package only describes
// its shape. See handler/handler.go for where these are actually used.
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

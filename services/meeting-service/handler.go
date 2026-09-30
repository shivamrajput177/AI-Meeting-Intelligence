// handler.go is Meeting Service's REST API — see
// docs/architecture/microservices.md §5.
package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	createUploadIntent *usecase.CreateUploadIntentUseCase
	confirmUpload      *usecase.ConfirmUploadUseCase
	getMeeting         *usecase.GetMeetingUseCase
	listMeetings       *usecase.ListMeetingsUseCase
	updateStatus       *usecase.UpdateStatusUseCase
	deleteMeeting      *usecase.DeleteMeetingUseCase
	getParticipants    *usecase.GetParticipantsUseCase
}

func NewHandler(
	createUploadIntent *usecase.CreateUploadIntentUseCase,
	confirmUpload *usecase.ConfirmUploadUseCase,
	getMeeting *usecase.GetMeetingUseCase,
	listMeetings *usecase.ListMeetingsUseCase,
	updateStatus *usecase.UpdateStatusUseCase,
	deleteMeeting *usecase.DeleteMeetingUseCase,
	getParticipants *usecase.GetParticipantsUseCase,
) *Handler {
	return &Handler{
		createUploadIntent: createUploadIntent, confirmUpload: confirmUpload,
		getMeeting: getMeeting, listMeetings: listMeetings,
		updateStatus: updateStatus, deleteMeeting: deleteMeeting,
		getParticipants: getParticipants,
	}
}

// fail writes the shared {"error": {...}} envelope via apperr.Write —
// every handler method below calls this at each of its error returns
// instead of writing one itself.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	apperr.Write(w, r.Header.Get(reqctx.HeaderRequestID), err)
}

func toMeetingResponse(m *entity.Meeting) entity.MeetingResponse {
	resp := entity.MeetingResponse{
		ID: m.ID, OrgID: m.OrgID, Title: m.Title, CreatedBy: m.CreatedBy,
		Status: m.Status, SourceType: m.SourceType, DurationSeconds: m.DurationSeconds,
		CreatedAt: m.CreatedAt.Format(time.RFC3339), UpdatedAt: m.UpdatedAt.Format(time.RFC3339),
	}
	if m.StartedAt != nil {
		s := m.StartedAt.Format(time.RFC3339)
		resp.StartedAt = &s
	}
	return resp
}

func (h *Handler) CreateUploadIntent(w http.ResponseWriter, r *http.Request) {
	var req entity.CreateMeetingRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.createUploadIntent.CreateUploadIntent(r.Context(), entity.CreateUploadIntentInput{
		OrgID: reqctx.OrgID(r.Context()), CreatedBy: reqctx.UserID(r.Context()), Title: req.Title,
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusCreated, entity.CreateMeetingResponse{MeetingID: out.MeetingID, UploadURL: out.UploadURL})
}

func (h *Handler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	meeting, err := h.confirmUpload.ConfirmUpload(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toMeetingResponse(meeting))
}

func (h *Handler) GetMeeting(w http.ResponseWriter, r *http.Request) {
	meeting, err := h.getMeeting.GetMeeting(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toMeetingResponse(meeting))
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	meeting, err := h.getMeeting.GetMeeting(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"meetingId": meeting.ID, "status": meeting.Status, "updatedAt": meeting.UpdatedAt.Format(time.RFC3339),
	})
}

func (h *Handler) ListMeetings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if pageSize == 0 {
		pageSize = 20
	}

	items, total, err := h.listMeetings.ListMeetings(r.Context(), reqctx.OrgID(r.Context()), entity.ListFilter{Page: page, PageSize: pageSize})
	if err != nil {
		fail(w, r, err)
		return
	}
	resp := entity.ListMeetingsResponse{Page: page, PageSize: pageSize, Total: total}
	for _, m := range items {
		resp.Data = append(resp.Data, toMeetingResponse(m))
	}
	httpserver.JSON(w, http.StatusOK, resp)
}

// UpdateStatus is the Phase 1 "debug endpoint" for manually flipping a
// meeting's status — see docs/architecture/microservices.md §5 and
// usecase.UpdateStatusUseCase's doc comment. Restricted to the org owner:
// it's a stand-in for the real pipeline, not a feature end users get.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	if reqctx.Role(r.Context()) != "owner" {
		fail(w, r, apperr.Forbidden("only the organization owner can manually override meeting status in Phase 1"))
		return
	}
	var req entity.UpdateStatusRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		fail(w, r, err)
		return
	}
	meeting, err := h.updateStatus.UpdateStatus(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"), req.Status)
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toMeetingResponse(meeting))
}

// GetParticipantsInternal serves GET /internal/meetings/{id}/participants
// — Action Item Service's own read for best-guess owner matching (see
// docs/architecture/microservices.md §8). orgId travels as an explicit
// query parameter rather than reqctx.OrgID, the same "internal caller
// states its own org_id" pattern transcription-service's
// GetTranscriptInternal already uses, for the same reason: there's no
// gateway-set JWT/org context on a direct service-to-service call.
func (h *Handler) GetParticipantsInternal(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		fail(w, r, apperr.BadRequest("orgId query parameter is required"))
		return
	}
	participants, err := h.getParticipants.GetParticipants(r.Context(), orgID, r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	resp := make([]entity.ParticipantResponse, len(participants))
	for i, p := range participants {
		resp[i] = entity.ParticipantResponse{UserID: p.UserID, Email: p.Email, DisplayName: p.DisplayName}
	}
	httpserver.JSON(w, http.StatusOK, resp)
}

// GetMeetingInternal is the same read as GetMeeting, reachable at
// /internal/meetings/{id} instead — this is what Search Service (Phase
// 3.3/3.4) calls to resolve a meeting's title for a search result or RAG
// citation. Same "internal caller states its own org_id" pattern as
// GetParticipantsInternal above.
func (h *Handler) GetMeetingInternal(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		fail(w, r, apperr.BadRequest("orgId query parameter is required"))
		return
	}
	meeting, err := h.getMeeting.GetMeeting(r.Context(), orgID, r.PathValue("id"))
	if err != nil {
		fail(w, r, err)
		return
	}
	httpserver.JSON(w, http.StatusOK, toMeetingResponse(meeting))
}

// ListMeetingsInternal is the same list as ListMeetings, reachable at
// /internal/meetings instead — this is what Search Service's
// POST /search/reindex (full-org backfill) calls to enumerate every
// meeting an org has, since it runs as a service-to-service call with no
// user JWT to read a page/org from. pageSize is capped at
// ListMeetingsUseCase's own max (100) same as the public route; a org
// with more meetings than that needs more than one reindex page today —
// an acceptable dev-scale limit, not a design Search Service's caller
// needs to reason about differently than any other paginated list here.
func (h *Handler) ListMeetingsInternal(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		fail(w, r, apperr.BadRequest("orgId query parameter is required"))
		return
	}
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if pageSize == 0 {
		pageSize = 100
	}

	items, total, err := h.listMeetings.ListMeetings(r.Context(), orgID, entity.ListFilter{Page: page, PageSize: pageSize})
	if err != nil {
		fail(w, r, err)
		return
	}
	resp := entity.ListMeetingsResponse{Page: page, PageSize: pageSize, Total: total}
	for _, m := range items {
		resp.Data = append(resp.Data, toMeetingResponse(m))
	}
	httpserver.JSON(w, http.StatusOK, resp)
}

func (h *Handler) DeleteMeeting(w http.ResponseWriter, r *http.Request) {
	if err := h.deleteMeeting.DeleteMeeting(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id")); err != nil {
		fail(w, r, err)
		return
	}
	httpserver.NoContent(w)
}

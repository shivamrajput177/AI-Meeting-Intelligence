// handler.go is Meeting Service's REST API — see
// docs/architecture/microservices.md §5.
package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/domain"
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
}

func NewHandler(
	createUploadIntent *usecase.CreateUploadIntentUseCase,
	confirmUpload *usecase.ConfirmUploadUseCase,
	getMeeting *usecase.GetMeetingUseCase,
	listMeetings *usecase.ListMeetingsUseCase,
	updateStatus *usecase.UpdateStatusUseCase,
	deleteMeeting *usecase.DeleteMeetingUseCase,
) *Handler {
	return &Handler{
		createUploadIntent: createUploadIntent, confirmUpload: confirmUpload,
		getMeeting: getMeeting, listMeetings: listMeetings,
		updateStatus: updateStatus, deleteMeeting: deleteMeeting,
	}
}

func toMeetingResponse(m *domain.Meeting) entity.MeetingResponse {
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

func (h *Handler) CreateUploadIntent(w http.ResponseWriter, r *http.Request) error {
	var req entity.CreateMeetingRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	out, err := h.createUploadIntent.CreateUploadIntent(r.Context(), entity.CreateUploadIntentInput{
		OrgID: reqctx.OrgID(r.Context()), CreatedBy: reqctx.UserID(r.Context()), Title: req.Title,
	})
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusCreated, entity.CreateMeetingResponse{MeetingID: out.MeetingID, UploadURL: out.UploadURL})
	return nil
}

func (h *Handler) ConfirmUpload(w http.ResponseWriter, r *http.Request) error {
	meeting, err := h.confirmUpload.ConfirmUpload(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toMeetingResponse(meeting))
	return nil
}

func (h *Handler) GetMeeting(w http.ResponseWriter, r *http.Request) error {
	meeting, err := h.getMeeting.GetMeeting(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toMeetingResponse(meeting))
	return nil
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) error {
	meeting, err := h.getMeeting.GetMeeting(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{
		"meetingId": meeting.ID, "status": meeting.Status, "updatedAt": meeting.UpdatedAt.Format(time.RFC3339),
	})
	return nil
}

func (h *Handler) ListMeetings(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	if page == 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))
	if pageSize == 0 {
		pageSize = 20
	}

	items, total, err := h.listMeetings.ListMeetings(r.Context(), reqctx.OrgID(r.Context()), domain.ListFilter{Page: page, PageSize: pageSize})
	if err != nil {
		return err
	}
	resp := entity.ListMeetingsResponse{Page: page, PageSize: pageSize, Total: total}
	for _, m := range items {
		resp.Data = append(resp.Data, toMeetingResponse(m))
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// UpdateStatus is the Phase 1 "debug endpoint" for manually flipping a
// meeting's status — see docs/architecture/microservices.md §5 and
// usecase.UpdateStatusUseCase's doc comment. Restricted to the org owner:
// it's a stand-in for the real pipeline, not a feature end users get.
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) error {
	if reqctx.Role(r.Context()) != "owner" {
		return apperr.Forbidden("only the organization owner can manually override meeting status in Phase 1")
	}
	var req entity.UpdateStatusRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	meeting, err := h.updateStatus.UpdateStatus(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"), req.Status)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toMeetingResponse(meeting))
	return nil
}

func (h *Handler) DeleteMeeting(w http.ResponseWriter, r *http.Request) error {
	if err := h.deleteMeeting.DeleteMeeting(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id")); err != nil {
		return err
	}
	httpserver.NoContent(w)
	return nil
}

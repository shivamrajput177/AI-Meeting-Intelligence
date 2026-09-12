// Package http is Meeting Service's REST API — see
// docs/architecture/microservices.md §5.
package http

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/meetingsvc/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
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

type meetingResponse struct {
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

func toMeetingResponse(m *domain.Meeting) meetingResponse {
	resp := meetingResponse{
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

type createMeetingRequest struct {
	Title string `json:"title"`
}

type createMeetingResponse struct {
	MeetingID string `json:"meetingId"`
	UploadURL string `json:"uploadUrl"`
}

func (h *Handler) CreateUploadIntent(c *fiber.Ctx) error {
	var req createMeetingRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	out, err := h.createUploadIntent.Execute(c.UserContext(), usecase.CreateUploadIntentInput{
		OrgID: reqctx.OrgID(c.UserContext()), CreatedBy: reqctx.UserID(c.UserContext()), Title: req.Title,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(createMeetingResponse{MeetingID: out.MeetingID, UploadURL: out.UploadURL})
}

func (h *Handler) ConfirmUpload(c *fiber.Ctx) error {
	meeting, err := h.confirmUpload.Execute(c.UserContext(), reqctx.OrgID(c.UserContext()), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(toMeetingResponse(meeting))
}

func (h *Handler) GetMeeting(c *fiber.Ctx) error {
	meeting, err := h.getMeeting.Execute(c.UserContext(), reqctx.OrgID(c.UserContext()), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(toMeetingResponse(meeting))
}

func (h *Handler) GetStatus(c *fiber.Ctx) error {
	meeting, err := h.getMeeting.Execute(c.UserContext(), reqctx.OrgID(c.UserContext()), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"meetingId": meeting.ID, "status": meeting.Status, "updatedAt": meeting.UpdatedAt.Format(time.RFC3339)})
}

type listMeetingsResponse struct {
	Data     []meetingResponse `json:"data"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
	Total    int               `json:"total"`
}

func (h *Handler) ListMeetings(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize", "20"))

	items, total, err := h.listMeetings.Execute(c.UserContext(), reqctx.OrgID(c.UserContext()), domain.ListFilter{Page: page, PageSize: pageSize})
	if err != nil {
		return err
	}
	resp := listMeetingsResponse{Page: page, PageSize: pageSize, Total: total}
	for _, m := range items {
		resp.Data = append(resp.Data, toMeetingResponse(m))
	}
	return c.JSON(resp)
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

// UpdateStatus is the Phase 1 "debug endpoint" for manually flipping a
// meeting's status — see docs/architecture/microservices.md §5 and
// usecase.UpdateStatusUseCase's doc comment. Restricted to the org owner:
// it's a stand-in for the real pipeline, not a feature end users get.
func (h *Handler) UpdateStatus(c *fiber.Ctx) error {
	if reqctx.Role(c.UserContext()) != "owner" {
		return apperr.Forbidden("only the organization owner can manually override meeting status in Phase 1")
	}
	var req updateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	meeting, err := h.updateStatus.Execute(c.UserContext(), reqctx.OrgID(c.UserContext()), c.Params("id"), req.Status)
	if err != nil {
		return err
	}
	return c.JSON(toMeetingResponse(meeting))
}

func (h *Handler) DeleteMeeting(c *fiber.Ctx) error {
	if err := h.deleteMeeting.Execute(c.UserContext(), reqctx.OrgID(c.UserContext()), c.Params("id")); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

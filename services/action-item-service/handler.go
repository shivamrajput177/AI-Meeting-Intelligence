// handler.go is Action Item Service's REST API — see
// docs/architecture/microservices.md §8.
package main

import (
	"net/http"
	"strconv"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	getActionItem    *usecase.GetActionItemUseCase
	listByMeeting    *usecase.ListActionItemsByMeetingUseCase
	listActionItems  *usecase.ListActionItemsUseCase
	updateActionItem *usecase.UpdateActionItemUseCase
}

func NewHandler(
	getActionItem *usecase.GetActionItemUseCase,
	listByMeeting *usecase.ListActionItemsByMeetingUseCase,
	listActionItems *usecase.ListActionItemsUseCase,
	updateActionItem *usecase.UpdateActionItemUseCase,
) *Handler {
	return &Handler{
		getActionItem: getActionItem, listByMeeting: listByMeeting,
		listActionItems: listActionItems, updateActionItem: updateActionItem,
	}
}

func toActionItemResponse(it *entity.ActionItem) entity.ActionItemResponse {
	resp := entity.ActionItemResponse{
		ID: it.ID, MeetingID: it.MeetingID, Description: it.Description, Type: it.Type,
		OwnerUserID: it.OwnerUserID, OwnerRawName: it.OwnerRawName,
		Status: it.Status, Priority: it.Priority, JiraIssueKey: it.JiraIssueKey,
		ExtractedFromChunkID: it.ExtractedFromChunkID, Confidence: it.Confidence,
		CreatedAt: it.CreatedAt.Format(time.RFC3339), UpdatedAt: it.UpdatedAt.Format(time.RFC3339),
	}
	if it.DueDate != nil {
		d := it.DueDate.Format("2006-01-02")
		resp.DueDate = &d
	}
	return resp
}

func (h *Handler) GetActionItem(w http.ResponseWriter, r *http.Request) error {
	item, err := h.getActionItem.GetActionItem(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toActionItemResponse(item))
	return nil
}

func (h *Handler) ListActionItemsForMeeting(w http.ResponseWriter, r *http.Request) error {
	items, err := h.listByMeeting.ListByMeeting(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	resp := make([]entity.ActionItemResponse, len(items))
	for i, it := range items {
		resp[i] = toActionItemResponse(it)
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// ListActionItems serves the cross-meeting GET /action-items?owner=&
// status=&type=&dueBefore=&page=&pageSize= per
// docs/architecture/api-spec.md's Action Items table.
func (h *Handler) ListActionItems(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	filter := entity.ListActionItemsFilter{
		OwnerUserID: q.Get("owner"), Status: q.Get("status"), Type: q.Get("type"),
	}
	if v := q.Get("dueBefore"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return apperr.BadRequest("dueBefore must be an ISO date (YYYY-MM-DD)")
		}
		filter.DueBefore = &t
	}
	filter.Page, _ = strconv.Atoi(q.Get("page"))
	filter.PageSize, _ = strconv.Atoi(q.Get("pageSize"))

	items, total, err := h.listActionItems.List(r.Context(), reqctx.OrgID(r.Context()), filter)
	if err != nil {
		return err
	}
	resp := entity.ListActionItemsResponse{Page: filter.Page, PageSize: filter.PageSize, Total: total}
	for _, it := range items {
		resp.Data = append(resp.Data, toActionItemResponse(it))
	}
	if resp.Page < 1 {
		resp.Page = 1
	}
	if resp.PageSize < 1 {
		resp.PageSize = 20
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// UpdateActionItem covers status update and owner reassignment — see
// UpdateActionItemUseCase's doc comment for the owner-or-admin
// authorization check this delegates to.
func (h *Handler) UpdateActionItem(w http.ResponseWriter, r *http.Request) error {
	var req entity.UpdateActionItemRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Status != nil && !entity.ValidStatuses[*req.Status] {
		return apperr.BadRequest("invalid status")
	}
	input := entity.UpdateActionItemInput{Status: req.Status, OwnerUserID: req.OwnerUserID}
	if req.DueDate != nil {
		t, err := time.Parse("2006-01-02", *req.DueDate)
		if err != nil {
			return apperr.BadRequest("dueDate must be an ISO date (YYYY-MM-DD)")
		}
		input.DueDate = &t
	}

	item, err := h.updateActionItem.UpdateActionItem(
		r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"),
		reqctx.UserID(r.Context()), reqctx.Role(r.Context()), input,
	)
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toActionItemResponse(item))
	return nil
}

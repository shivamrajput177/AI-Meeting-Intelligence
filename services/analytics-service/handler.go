// handler.go is Analytics Service's REST API — see
// docs/architecture/microservices.md §11. Every response is a
// pre-aggregated read off a rollup table (CQRS read-side), never a join
// against another service's operational tables.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	trends         *usecase.GetMeetingTrendsUseCase
	productivity   *usecase.GetProductivityUseCase
	completionRate *usecase.GetCompletionRateUseCase
	topics         *usecase.GetTopicsUseCase
}

func NewHandler(
	trends *usecase.GetMeetingTrendsUseCase,
	productivity *usecase.GetProductivityUseCase,
	completionRate *usecase.GetCompletionRateUseCase,
	topics *usecase.GetTopicsUseCase,
) *Handler {
	return &Handler{trends: trends, productivity: productivity, completionRate: completionRate, topics: topics}
}

func (h *Handler) MeetingTrends(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.trends.GetTrends(r.Context(), reqctx.OrgID(r.Context()))
	if err != nil {
		return err
	}
	resp := entity.MeetingTrendsResponse{}
	for _, row := range rows {
		resp.Data = append(resp.Data, entity.MeetingTrendPoint{
			Day: row.Day.Format("2006-01-02"), MeetingCount: row.MeetingCount, TotalMinutes: row.TotalMinutes,
		})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) Productivity(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.productivity.GetProductivity(r.Context(), reqctx.OrgID(r.Context()))
	if err != nil {
		return err
	}
	resp := entity.ProductivityResponse{}
	for _, row := range rows {
		resp.Data = append(resp.Data, entity.ProductivityEntry{OwnerUserID: row.OwnerUserID, Opened: row.Opened, Closed: row.Closed})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) CompletionRate(w http.ResponseWriter, r *http.Request) error {
	opened, closed, rate, err := h.completionRate.GetCompletionRate(r.Context(), reqctx.OrgID(r.Context()))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, entity.CompletionRateResponse{OpenedTotal: opened, ClosedTotal: closed, CompletionRate: rate})
	return nil
}

func (h *Handler) Topics(w http.ResponseWriter, r *http.Request) error {
	rows, err := h.topics.GetTopics(r.Context(), reqctx.OrgID(r.Context()))
	if err != nil {
		return err
	}
	resp := entity.TopicsResponse{}
	for _, row := range rows {
		resp.Data = append(resp.Data, entity.TopicEntry{Topic: row.Topic, Mentions: row.Mentions})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

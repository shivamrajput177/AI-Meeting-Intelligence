// handler.go is Search Service's REST API — see
// docs/architecture/microservices.md §9.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	search  *usecase.SearchUseCase
	similar *usecase.SimilarMeetingsUseCase
	ask     *usecase.AskUseCase
	history *usecase.GetHistoryUseCase
	reindex *usecase.ReindexUseCase
}

func NewHandler(
	search *usecase.SearchUseCase, similar *usecase.SimilarMeetingsUseCase,
	ask *usecase.AskUseCase, history *usecase.GetHistoryUseCase, reindex *usecase.ReindexUseCase,
) *Handler {
	return &Handler{search: search, similar: similar, ask: ask, history: history, reindex: reindex}
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) error {
	query := r.URL.Query().Get("q")
	if query == "" {
		return apperr.BadRequest("q query parameter is required")
	}
	results, err := h.search.Search(r.Context(), reqctx.OrgID(r.Context()), query)
	if err != nil {
		return err
	}
	resp := entity.SearchResponse{Query: query}
	for _, res := range results {
		resp.Results = append(resp.Results, entity.SearchResultResponse{
			ChunkID: res.Hit.ChunkID, MeetingID: res.Hit.MeetingID, MeetingTitle: res.MeetingTitle,
			Text: res.Hit.Text, StartMs: res.Hit.StartMS, EndMs: res.Hit.EndMS, Score: res.Hit.Score,
		})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) SimilarMeetings(w http.ResponseWriter, r *http.Request) error {
	results, err := h.similar.Similar(r.Context(), reqctx.OrgID(r.Context()), r.PathValue("id"))
	if err != nil {
		return err
	}
	resp := entity.SimilarMeetingsResponse{}
	for _, res := range results {
		resp.Results = append(resp.Results, entity.SimilarMeetingResponse{
			MeetingID: res.Meeting.MeetingID, MeetingTitle: res.Title, Score: res.Meeting.Score,
		})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) Ask(w http.ResponseWriter, r *http.Request) error {
	var req entity.AskRequest
	if err := httpserver.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Question == "" {
		return apperr.BadRequest("question is required")
	}

	result, err := h.ask.Ask(r.Context(), reqctx.OrgID(r.Context()), reqctx.UserID(r.Context()), req.Question)
	if err != nil {
		return err
	}
	resp := entity.AskResponse{Answer: result.Answer}
	for _, c := range result.Citations {
		resp.Citations = append(resp.Citations, entity.CitationResponse{
			MeetingID: c.MeetingID, MeetingTitle: c.MeetingTitle, StartMs: c.StartMS, EndMs: c.EndMS, Snippet: c.Snippet,
		})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

func (h *Handler) GetHistory(w http.ResponseWriter, r *http.Request) error {
	entries, err := h.history.GetHistory(r.Context(), reqctx.OrgID(r.Context()), reqctx.UserID(r.Context()))
	if err != nil {
		return err
	}
	resp := entity.QAHistoryResponse{}
	for _, e := range entries {
		resp.Data = append(resp.Data, entity.QAHistoryEntryResponse{
			ID: e.ID, Question: e.Question, Answer: e.Answer,
			CitedChunkIDs: e.CitedChunkIDs, CreatedAt: e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	httpserver.JSON(w, http.StatusOK, resp)
	return nil
}

// Reindex is admin-only — see routes.go's RequireRole gate, the
// per-service half of the defense-in-depth the gateway's own RequireRole
// is the other half of (docs/architecture/observability-security.md §2).
func (h *Handler) Reindex(w http.ResponseWriter, r *http.Request) error {
	result, err := h.reindex.Reindex(r.Context(), reqctx.OrgID(r.Context()))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, entity.ReindexResponse{
		MeetingsReindexed: result.MeetingsReindexed,
		MeetingsFailed:    len(result.FailedMeetingIDs),
		FailedMeetingIDs:  result.FailedMeetingIDs,
	})
	return nil
}

// handler.go is AI Summary Service's REST API — see
// docs/architecture/microservices.md §7.
package main

import (
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	getSummary        *usecase.GetSummaryUseCase
	processTranscript *usecase.ProcessTranscriptUseCase
}

func NewHandler(getSummary *usecase.GetSummaryUseCase, processTranscript *usecase.ProcessTranscriptUseCase) *Handler {
	return &Handler{getSummary: getSummary, processTranscript: processTranscript}
}

func toSummaryResponse(s *entity.Summary) entity.SummaryResponse {
	return entity.SummaryResponse{
		ID: s.ID, MeetingID: s.MeetingID, SummaryText: s.SummaryText,
		KeyDecisions: s.KeyDecisions, Risks: s.Risks, Blockers: s.Blockers,
		ModelUsed: s.ModelUsed, PromptVersion: s.PromptVersion,
		CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
}

func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) error {
	orgID := reqctx.OrgID(r.Context())
	summary, err := h.getSummary.GetSummary(r.Context(), orgID, r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toSummaryResponse(summary))
	return nil
}

// RegenerateSummary re-runs the exact same fetch-chunk-summarize-persist
// pipeline consumer.go's Kafka path runs, synchronously, on demand — see
// ProcessTranscriptUseCase's doc comment. There's no separate
// "regenerate" usecase: this is the same pipeline, just triggered by a
// person instead of an event.
func (h *Handler) RegenerateSummary(w http.ResponseWriter, r *http.Request) error {
	orgID := reqctx.OrgID(r.Context())
	summary, err := h.processTranscript.ProcessTranscript(r.Context(), orgID, r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toSummaryResponse(summary))
	return nil
}

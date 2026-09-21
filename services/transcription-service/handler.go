// handler.go is Transcription Service's REST API — see
// docs/architecture/microservices.md §6.
package main

import (
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

type Handler struct {
	getTranscript *usecase.GetTranscriptUseCase
}

func NewHandler(getTranscript *usecase.GetTranscriptUseCase) *Handler {
	return &Handler{getTranscript: getTranscript}
}

func toTranscriptResponse(t *entity.Transcript, segments []*entity.Segment) entity.TranscriptResponse {
	resp := entity.TranscriptResponse{
		ID: t.ID, MeetingID: t.MeetingID, Language: t.Language, Engine: t.Engine,
		ModelName: t.ModelName, Status: t.Status, RawText: t.RawText, WordCount: t.WordCount,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
	for _, s := range segments {
		resp.Segments = append(resp.Segments, entity.SegmentResponse{
			SpeakerLabel: s.SpeakerLabel, StartMs: s.StartMS, EndMs: s.EndMS,
			Text: s.Text, Confidence: s.Confidence,
		})
	}
	return resp
}

// GetTranscript serves the gateway-fronted GET /meetings/{id}/transcript.
func (h *Handler) GetTranscript(w http.ResponseWriter, r *http.Request) error {
	orgID := reqctx.OrgID(r.Context())
	transcript, segments, err := h.getTranscript.GetTranscript(r.Context(), orgID, r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toTranscriptResponse(transcript, segments))
	return nil
}

// GetTranscriptInternal is the same read, reachable at
// /internal/meetings/{id}/transcript instead — this is what AI Summary
// Service (Phase 2.4) actually calls: "calls GET /meetings/{id}/transcript
// on Transcription Service over REST" per
// docs/architecture/microservices.md §7, but that's a direct
// service-to-service call with no gateway-set JWT/org context to read
// reqctx.OrgID from. orgId travels as an explicit query parameter
// instead, the same "internal caller states its own org_id" pattern
// user-service's CreateUserRequest.OrgID already uses for auth-service's
// signup call — trusted at face value once RequireInternalToken has
// already verified the caller is a legitimate internal service (see that
// middleware's own doc comment on this project's Phase 1 trust model).
func (h *Handler) GetTranscriptInternal(w http.ResponseWriter, r *http.Request) error {
	orgID := r.URL.Query().Get("orgId")
	if orgID == "" {
		return apperr.BadRequest("orgId query parameter is required")
	}
	transcript, segments, err := h.getTranscript.GetTranscript(r.Context(), orgID, r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toTranscriptResponse(transcript, segments))
	return nil
}

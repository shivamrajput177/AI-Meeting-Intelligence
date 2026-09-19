// handler.go is Transcription Service's REST API — see
// docs/architecture/microservices.md §6.
package main

import (
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/transcription-service/usecase"
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
// AI Summary Service (Phase 2.4) is documented to read a transcript the
// same way — "calls GET /meetings/{id}/transcript on Transcription
// Service over REST" — but that's a direct service-to-service call with
// no gateway-set org context, which this handler doesn't yet handle (it
// only ever reads reqctx.OrgID, same as meeting-service's GetMeeting).
// Left as a known, explicitly-called-out gap rather than solved
// speculatively: there's no caller to get it right for until 2.4 exists.
func (h *Handler) GetTranscript(w http.ResponseWriter, r *http.Request) error {
	orgID := reqctx.OrgID(r.Context())
	transcript, segments, err := h.getTranscript.GetTranscript(r.Context(), orgID, r.PathValue("id"))
	if err != nil {
		return err
	}
	httpserver.JSON(w, http.StatusOK, toTranscriptResponse(transcript, segments))
	return nil
}

// routes.go mounts Transcription Service's Handler methods onto a
// *http.ServeMux — kept as its own file, not folded into handler.go, so
// "what each route does" (handler.go) and "which path/method maps to
// which method" (this file) stay two things you can read independently
// even though they're one package now.
package main

import (
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpserver"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.Handle("GET /meetings/{id}/transcript", httpserver.H(h.GetTranscript))
}

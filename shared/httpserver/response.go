package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/reqctx"
)

// HandlerFunc is a request handler that returns an error instead of
// writing one itself — the net/http equivalent of Fiber's
// func(*fiber.Ctx) error style, kept because "return the error and let
// one place format it" is worth preserving even without the framework.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

// H adapts a HandlerFunc to a plain http.Handler for mux.Handle: on error,
// it writes the shared {"error": {...}} envelope via apperr.Write instead
// of every handler doing that itself.
func H(fn HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			apperr.Write(w, r.Header.Get(reqctx.HeaderRequestID), err)
		}
	})
}

// JSON writes v as a JSON response with the given status code — the
// stdlib equivalent of Fiber's c.Status(status).JSON(v).
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NoContent writes a 204 with no body — the stdlib equivalent of Fiber's
// c.SendStatus(fiber.StatusNoContent).
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// DecodeJSON reads and JSON-decodes the request body into v, returning a
// well-formed apperr.BadRequest on failure instead of a raw decode error
// — the stdlib equivalent of Fiber's c.BodyParser(&req).
func DecodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	return nil
}

// Package apperr defines the typed application error every usecase layer
// returns, and the single place that maps it to an HTTP status + the
// response envelope documented in docs/architecture/api-spec.md:
//
//	{"error": {"code": "string", "message": "string", "requestId": "uuid"}}
package apperr

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v2"
)

// Error is a typed application error carrying an HTTP status and a stable
// machine-readable code, distinct from both the raw Go error (which may
// leak internals) and the user-facing message.
type Error struct {
	Status  int
	Code    string
	Message string
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.cause }

// Wrap attaches a lower-level cause to an *Error for logging, without
// changing the message returned to the client.
func (e *Error) Wrap(cause error) *Error {
	return &Error{Status: e.Status, Code: e.Code, Message: e.Message, cause: cause}
}

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "forbidden", message)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, "not_found", message)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "conflict", message)
}

func TooManyRequests(message string) *Error {
	return New(http.StatusTooManyRequests, "rate_limited", message)
}

func Internal(message string) *Error {
	return New(http.StatusInternalServerError, "internal_error", message)
}

// AsError extracts an *Error from err, falling back to a generic 500 if
// err is nil, not an *Error, or is a wrapped context/deadline error.
func AsError(err error) *Error {
	if err == nil {
		return nil
	}
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return Internal("unexpected error")
}

// FiberHandler is the single place an HTTP handler turns a usecase error
// into a response — every delivery/http package in this repo calls this
// instead of writing its own error JSON.
func FiberHandler(c *fiber.Ctx, err error) error {
	appErr := AsError(err)
	requestID := c.Locals("requestid")
	return c.Status(appErr.Status).JSON(fiber.Map{
		"error": fiber.Map{
			"code":      appErr.Code,
			"message":   appErr.Message,
			"requestId": requestID,
		},
	})
}

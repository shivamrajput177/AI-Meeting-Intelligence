// Package httpclient is the one client every service uses to call another
// service's REST API directly — the internal-communication mechanism this
// design uses instead of gRPC (see
// docs/architecture/microservices.md §"Internal Communication").
//
// Phase 1 gives it a timeout, one retry on network-level failure, and
// request-id propagation. A real circuit breaker (trip after N
// consecutive failures, half-open probing) is documented as a Phase 2+
// hardening item once there's an actual failure-prone dependency
// (Ollama/whisper.cpp) to justify it — see
// docs/architecture/microservices.md §7-9.
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/reqctx"
)

type Client struct {
	baseURL       string
	internalToken string
	hc            *http.Client
}

// New returns a client for calling the service at baseURL (e.g.
// "http://user-service:8080"). internalToken is sent as X-Internal-Token
// on every request — required by any service's /internal/* routes and
// harmlessly ignored by its public ones.
func New(baseURL, internalToken string) *Client {
	return &Client{
		baseURL:       baseURL,
		internalToken: internalToken,
		hc:            &http.Client{Timeout: 5 * time.Second},
	}
}

// Do sends a JSON request and decodes a JSON response into out (which may
// be nil for calls with no response body). It carries the caller's
// request id and, if present in ctx, org/user/role context — the same
// headers the gateway sets when it authenticates a request — so a call
// chain stays attributable to one tenant end to end.
func (c *Client) Do(ctx context.Context, method, path string, in, out interface{}) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return apperr.Internal("marshal request").Wrap(err)
		}
		body = bytes.NewReader(b)
	}

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return apperr.Internal("build request").Wrap(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(reqctx.HeaderInternalToken, c.internalToken)
		if id := reqctx.RequestID(ctx); id != "" {
			req.Header.Set(reqctx.HeaderRequestID, id)
		}
		if org := reqctx.OrgID(ctx); org != "" {
			req.Header.Set(reqctx.HeaderOrgID, org)
		}
		if user := reqctx.UserID(ctx); user != "" {
			req.Header.Set(reqctx.HeaderUserID, user)
		}
		if role := reqctx.Role(ctx); role != "" {
			req.Header.Set(reqctx.HeaderRole, role)
		}

		resp, err := c.hc.Do(req)
		if err != nil {
			lastErr = err
			if attempt == 1 {
				time.Sleep(150 * time.Millisecond)
				continue
			}
			return apperr.Internal(fmt.Sprintf("calling %s", path)).Wrap(err)
		}
		defer func() { _ = resp.Body.Close() }()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return apperr.Internal("read response").Wrap(err)
		}

		if resp.StatusCode >= 400 {
			return mapErrorResponse(resp.StatusCode, respBody)
		}
		if out != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return apperr.Internal("decode response").Wrap(err)
			}
		}
		return nil
	}
	return apperr.Internal(fmt.Sprintf("calling %s", path)).Wrap(lastErr)
}

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// mapErrorResponse turns another service's {"error": {...}} JSON body
// (the shared envelope from internal/platform/apperr) back into an
// *apperr.Error, preserving the original status code so, e.g., a 404 from
// User Service surfaces as a 404 to the client calling through Auth
// Service, not a generic 500.
func mapErrorResponse(status int, body []byte) error {
	var env errorEnvelope
	msg := "internal service error"
	code := "internal_error"
	if json.Unmarshal(body, &env) == nil && env.Error.Message != "" {
		msg = env.Error.Message
		code = env.Error.Code
	}
	return apperr.New(status, code, msg)
}

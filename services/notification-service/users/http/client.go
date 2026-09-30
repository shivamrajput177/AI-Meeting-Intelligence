// Package http implements users.Client against User Service's internal
// REST API (GET /internal/users/{id} — added alongside this service, see
// that handler's own doc comment). Built on shared/httpclient, same as
// every other internal-call client in this repo.
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// GetEmail passes orgID as an explicit query parameter, not a header —
// see usersvc's GetUserInternal handler's doc comment for why: there's no
// gateway-set JWT/org context on this service-to-service call, so the
// caller (this client) states its own org_id explicitly, the same pattern
// this project's other internal clients already use.
func (c *Client) GetEmail(ctx context.Context, orgID, userID string) (string, error) {
	var resp entity.UserWireResponse
	path := "/internal/users/" + userID + "?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return "", err
	}
	return resp.Email, nil
}

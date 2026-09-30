// Package http implements meetings.Client against Meeting Service's
// internal REST API (GET /internal/meetings/{id}) — this service's
// enqueue usecases need a meeting's title (for message text) and creator
// (to resolve an email recipient), neither of which either Kafka event it
// consumes carries. Built on shared/httpclient, same as every other
// internal-call client in this repo.
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// GetMeeting passes orgID as an explicit query parameter, not a header —
// see meetingsvc's GetMeetingInternal handler's doc comment for why:
// there's no gateway-set JWT/org context on this service-to-service call,
// so the caller (this client) states its own org_id explicitly, the same
// pattern this project's other internal clients already use.
func (c *Client) GetMeeting(ctx context.Context, orgID, meetingID string) (*meetings.Meeting, error) {
	var resp entity.MeetingWireResponse
	path := "/internal/meetings/" + meetingID + "?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &meetings.Meeting{Title: resp.Title, CreatedBy: resp.CreatedBy}, nil
}

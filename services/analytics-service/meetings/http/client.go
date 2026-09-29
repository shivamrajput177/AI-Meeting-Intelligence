// Package http implements meetings.Client against Meeting Service's
// internal REST API (GET /internal/meetings/{id}) — this service's own
// per-meeting-completion rollup needs a meeting's duration, which
// meeting.status-changed.v1 itself doesn't carry (see
// entity.MeetingStatusChangedEvent). Built on shared/httpclient, same as
// every other internal-call client in this repo.
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// GetDurationMinutes passes orgID as an explicit query parameter, not a
// header — see meetingsvc's GetMeetingInternal handler's doc comment for
// why: there's no gateway-set JWT/org context on this service-to-service
// call, so the caller (this client) states its own org_id explicitly, the
// same pattern this project's other internal clients already use.
func (c *Client) GetDurationMinutes(ctx context.Context, orgID, meetingID string) (int, error) {
	var resp entity.MeetingWireResponse
	path := "/internal/meetings/" + meetingID + "?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return 0, err
	}
	if resp.DurationSeconds == nil {
		return 0, nil
	}
	return *resp.DurationSeconds / 60, nil
}

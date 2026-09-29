// Package http implements actionitems.Client against Action Item
// Service's internal REST API (GET /internal/meetings/{id}/action-items —
// added alongside this service, see that handler's own doc comment).
// Built on shared/httpclient, same as every other internal-call client in
// this repo.
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/actionitems"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// ListForMeeting passes orgID as an explicit query parameter, not a
// header — see actionitemsvc's ListActionItemsForMeetingInternal
// handler's doc comment for why: there's no gateway-set JWT/org context on
// this service-to-service call, so the caller (this client) states its
// own org_id explicitly, the same pattern this project's other internal
// clients already use.
func (c *Client) ListForMeeting(ctx context.Context, orgID, meetingID string) ([]actionitems.Item, error) {
	var resp []entity.ActionItemWireResponse
	path := "/internal/meetings/" + meetingID + "/action-items?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	items := make([]actionitems.Item, len(resp))
	for i, it := range resp {
		items[i] = actionitems.Item{ID: it.ID, OwnerUserID: it.OwnerUserID}
	}
	return items, nil
}

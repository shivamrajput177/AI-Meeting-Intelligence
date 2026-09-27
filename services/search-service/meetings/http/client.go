// Package http implements meetings.Client against Meeting Service's
// internal REST API (GET /internal/meetings/{id}, GET /internal/meetings)
// — see docs/architecture/microservices.md §9: search results and RAG
// citations need a meeting's title, not just its id. Built on
// shared/httpclient, same as every other internal-call client in this
// repo.
package http

import (
	"context"
	"net/url"
	"strconv"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
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
func (c *Client) GetMeeting(ctx context.Context, orgID, meetingID string) (*entity.Meeting, error) {
	var resp entity.MeetingWireResponse
	path := "/internal/meetings/" + meetingID + "?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &entity.Meeting{ID: resp.ID, Title: resp.Title}, nil
}

func (c *Client) ListMeetings(ctx context.Context, orgID string, page, pageSize int) ([]entity.Meeting, int, error) {
	var resp entity.MeetingListWireResponse
	path := "/internal/meetings?orgId=" + url.QueryEscape(orgID) + "&page=" + strconv.Itoa(page) + "&pageSize=" + strconv.Itoa(pageSize)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, 0, err
	}
	meetings := make([]entity.Meeting, len(resp.Data))
	for i, m := range resp.Data {
		meetings[i] = entity.Meeting(m)
	}
	return meetings, resp.Total, nil
}

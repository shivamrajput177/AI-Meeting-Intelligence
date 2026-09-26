// Package http implements participants.Client against Meeting Service's
// internal REST API (GET /internal/meetings/{id}/participants) — see
// docs/architecture/microservices.md §8: owner matching is done "against
// meeting.participants". Built on shared/httpclient, same as every other
// internal-call client in this repo.
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// ListParticipants passes orgID as an explicit query parameter, not a
// header — see meetingsvc's GetParticipantsInternal handler's doc comment
// for why: there's no gateway-set JWT/org context on this
// service-to-service call, so the caller (this client) states its own
// org_id explicitly, the same pattern this project's other internal
// clients already use.
func (c *Client) ListParticipants(ctx context.Context, orgID, meetingID string) ([]entity.Participant, error) {
	var resp []entity.ParticipantWireResponse
	path := "/internal/meetings/" + meetingID + "/participants?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	items := make([]entity.Participant, len(resp))
	for i, p := range resp {
		items[i] = entity.Participant(p)
	}
	return items, nil
}

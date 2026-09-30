// Package http implements actionitems.Client against Action Item
// Service's internal REST API (PATCH /internal/action-items/{id} — added
// alongside this service, see that handler's own doc comment). Built on
// shared/httpclient, same as every other internal-call client in this
// repo.
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

// UpdateActionItem passes orgID as an explicit query parameter, not a
// header — see actionitemsvc's UpdateActionItemInternal handler's doc
// comment for why: there's no gateway-set JWT/org context on this
// service-to-service call, so the caller (this client) states its own
// org_id explicitly, the same pattern this project's other internal
// clients already use.
func (c *Client) UpdateActionItem(ctx context.Context, orgID, actionItemID string, status, jiraIssueKey *string) error {
	path := "/internal/action-items/" + actionItemID + "?orgId=" + url.QueryEscape(orgID)
	req := entity.UpdateActionItemInternalRequest{Status: status, JiraIssueKey: jiraIssueKey}
	return c.http.Do(ctx, "PATCH", path, req, nil)
}

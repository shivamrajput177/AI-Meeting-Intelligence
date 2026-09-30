// Package http implements orgs.Client against Organization Service's
// internal REST API (GET /internal/orgs/{orgId}/integration-config) — the
// per-org override DispatchUseCase checks before falling back to this
// service's own dev-config-wide Slack webhook URL and mock ticket
// provider (see main.go's doc comment on that Phase 4.1 gap, closed in
// Phase 4.5). Built on shared/httpclient, same as every other
// internal-call client in this repo.
package http

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/orgs"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// GetIntegrationConfig passes orgID as an explicit path segment, not a
// header — same reason as every other internal client in this repo (see
// e.g. meetings/http.Client.GetMeeting's doc comment): there's no
// gateway-set JWT/org context on this service-to-service call.
func (c *Client) GetIntegrationConfig(ctx context.Context, orgID string) (*orgs.IntegrationConfig, error) {
	var resp entity.OrgIntegrationConfigWireResponse
	path := "/internal/orgs/" + orgID + "/integration-config"
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &orgs.IntegrationConfig{SlackWebhookURL: resp.SlackWebhookURL, TicketProvider: resp.TicketProvider}, nil
}

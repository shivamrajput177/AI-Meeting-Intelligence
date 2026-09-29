// Package http implements summary.Client against AI Summary Service's
// internal REST API (GET /internal/meetings/{id}/summary — the same
// endpoint Action Item Service's own summary client already calls). Built
// on shared/httpclient, same as every other internal-call client in this
// repo.
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/analytics-service/summary"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// GetSummary passes orgID as an explicit query parameter, not a header —
// see aisummarysvc's GetSummaryInternal handler's doc comment for why:
// there's no gateway-set JWT/org context on this service-to-service call,
// so the caller (this client) states its own org_id explicitly, the same
// pattern this project's other internal clients already use.
func (c *Client) GetSummary(ctx context.Context, orgID, meetingID string) (*summary.Summary, error) {
	var resp entity.SummaryWireResponse
	path := "/internal/meetings/" + meetingID + "/summary?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	return &summary.Summary{KeyDecisions: resp.KeyDecisions, Risks: resp.Risks, Blockers: resp.Blockers}, nil
}

// Package http implements chunks.Client against AI Summary Service's
// internal REST API (GET /internal/meetings/{id}/chunks) — see
// docs/architecture/microservices.md §9: this service's embedding
// pipeline needs the chunk text chunk.created.v1 deliberately doesn't
// carry. Built on shared/httpclient, same as every other internal-call
// client in this repo.
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// ListChunks passes orgID as an explicit query parameter, not a header —
// see aisummarysvc's GetChunksInternal handler's doc comment for why:
// there's no gateway-set JWT/org context on this service-to-service call,
// so the caller (this client) states its own org_id explicitly, the same
// pattern this project's other internal clients already use.
func (c *Client) ListChunks(ctx context.Context, orgID, meetingID string) ([]entity.Chunk, error) {
	var resp []entity.ChunkWireResponse
	path := "/internal/meetings/" + meetingID + "/chunks?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	chunks := make([]entity.Chunk, len(resp))
	for i, c := range resp {
		chunks[i] = entity.Chunk{ID: c.ID, MeetingID: c.MeetingID, Text: c.Text, StartMS: c.StartMs, EndMS: c.EndMs}
	}
	return chunks, nil
}

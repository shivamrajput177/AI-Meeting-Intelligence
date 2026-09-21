// Package http implements transcript.Client against Transcription
// Service's internal REST API (GET /internal/meetings/{id}/transcript) —
// see docs/architecture/microservices.md §7: "this service then calls
// GET /meetings/{id}/transcript on Transcription Service over REST to
// fetch the actual text." Built on shared/httpclient, same as every
// other internal-call client in this repo (e.g. auth-service's
// OrgClient/UserClient).
package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type Client struct {
	http *httpclient.Client
}

func NewClient(baseURL, internalToken string) *Client {
	return &Client{http: httpclient.New(baseURL, internalToken)}
}

// GetTranscript passes orgID as an explicit query parameter, not a
// header — see transcriptionsvc's GetTranscriptInternal handler's doc
// comment for why: there's no gateway-set JWT/org context on this
// service-to-service call, so the caller (this client) states its own
// org_id explicitly, the same pattern user-service's
// CreateUserRequest.OrgID already uses for auth-service's signup call.
func (c *Client) GetTranscript(ctx context.Context, orgID, meetingID string) (*entity.Transcript, error) {
	var resp entity.TranscriptWireResponse
	path := "/internal/meetings/" + meetingID + "/transcript?orgId=" + url.QueryEscape(orgID)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}

	transcript := &entity.Transcript{MeetingID: resp.MeetingID, RawText: resp.RawText}
	for _, s := range resp.Segments {
		transcript.Segments = append(transcript.Segments, entity.TranscriptSegment{
			SpeakerLabel: s.SpeakerLabel, StartMS: s.StartMs, EndMS: s.EndMs, Text: s.Text,
		})
	}
	return transcript, nil
}

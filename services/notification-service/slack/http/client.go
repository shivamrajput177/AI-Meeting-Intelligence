// Package http implements slack.Sender against a real Slack Incoming
// Webhook (https://api.slack.com/messaging/webhooks) — a plain
// http.Client, not shared/httpclient, since a webhook URL is an external
// third-party endpoint, not another service in this repo (no
// X-Internal-Token/org/user header propagation applies).
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	http *http.Client
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 10 * time.Second}}
}

type webhookRequest struct {
	Text string `json:"text"`
}

// Send posts {"text": text} to webhookURL, the whole of Slack's Incoming
// Webhook request contract for a plain text message (no Block Kit
// formatting — nothing here needs it yet).
func (c *Client) Send(ctx context.Context, webhookURL, text string) error {
	body, err := json.Marshal(webhookRequest{Text: text})
	if err != nil {
		return fmt.Errorf("slack: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("slack: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("slack: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack: webhook returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

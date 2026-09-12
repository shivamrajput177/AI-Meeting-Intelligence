// Package client implements authsvc/domain's OrgClient/UserClient ports
// against the real internal REST APIs, via the shared
// internal/platform/httpclient — this is the concrete "how" behind the
// abstract ports the usecase layer depends on.
package client

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpclient"
)

type OrgClient struct {
	http *httpclient.Client
}

func NewOrgClient(baseURL, internalToken string) *OrgClient {
	return &OrgClient{http: httpclient.New(baseURL, internalToken)}
}

type createOrgRequest struct {
	Name string `json:"name"`
}

type orgResponse struct {
	ID string `json:"id"`
}

func (c *OrgClient) CreateOrg(ctx context.Context, name string) (string, error) {
	var resp orgResponse
	if err := c.http.Do(ctx, "POST", "/internal/orgs", createOrgRequest{Name: name}, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

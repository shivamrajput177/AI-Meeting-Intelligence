// Package http implements client.OrgClient/UserClient against the real
// internal REST APIs, via the shared shared/httpclient — this is the
// concrete "how" behind the abstract ports the usecase layer depends on.
package http

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type OrgClient struct {
	http *httpclient.Client
}

func NewOrgClient(baseURL, internalToken string) *OrgClient {
	return &OrgClient{http: httpclient.New(baseURL, internalToken)}
}

func (c *OrgClient) CreateOrg(ctx context.Context, name string) (string, error) {
	var resp entity.OrgResponse
	if err := c.http.Do(ctx, "POST", "/internal/orgs", entity.CreateOrgRequest{Name: name}, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

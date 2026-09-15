package http

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/auth-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/httpclient"
)

type UserClient struct {
	http *httpclient.Client
}

func NewUserClient(baseURL, internalToken string) *UserClient {
	return &UserClient{http: httpclient.New(baseURL, internalToken)}
}

func (c *UserClient) CreateUser(ctx context.Context, orgID, email, name, role string) (string, error) {
	var resp entity.UserResponse
	if err := c.http.Do(ctx, "POST", "/internal/users",
		entity.CreateUserRequest{OrgID: orgID, Email: email, Name: name, Role: role}, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *UserClient) LookupByEmail(ctx context.Context, email string) ([]entity.EmailMatch, error) {
	var resp entity.LookupResponse
	path := "/internal/users/lookup?email=" + url.QueryEscape(email)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	matches := make([]entity.EmailMatch, len(resp.Matches))
	for i, m := range resp.Matches {
		matches[i] = entity.EmailMatch(m)
	}
	return matches, nil
}

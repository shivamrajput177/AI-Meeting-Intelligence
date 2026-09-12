package client

import (
	"context"
	"net/url"

	"github.com/shivamrajput177/ai-meeting-intelligence/internal/authsvc/domain"
	"github.com/shivamrajput177/ai-meeting-intelligence/internal/platform/httpclient"
)

type UserClient struct {
	http *httpclient.Client
}

func NewUserClient(baseURL, internalToken string) *UserClient {
	return &UserClient{http: httpclient.New(baseURL, internalToken)}
}

type createUserRequest struct {
	OrgID string `json:"orgId"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type userResponse struct {
	ID string `json:"id"`
}

func (c *UserClient) CreateUser(ctx context.Context, orgID, email, name, role string) (string, error) {
	var resp userResponse
	if err := c.http.Do(ctx, "POST", "/internal/users",
		createUserRequest{OrgID: orgID, Email: email, Name: name, Role: role}, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

type lookupResponse struct {
	Matches []struct {
		UserID string `json:"userId"`
		OrgID  string `json:"orgId"`
		Role   string `json:"role"`
		Status string `json:"status"`
	} `json:"matches"`
}

func (c *UserClient) LookupByEmail(ctx context.Context, email string) ([]domain.EmailMatch, error) {
	var resp lookupResponse
	path := "/internal/users/lookup?email=" + url.QueryEscape(email)
	if err := c.http.Do(ctx, "GET", path, nil, &resp); err != nil {
		return nil, err
	}
	matches := make([]domain.EmailMatch, len(resp.Matches))
	for i, m := range resp.Matches {
		matches[i] = domain.EmailMatch{UserID: m.UserID, OrgID: m.OrgID, Role: m.Role, Status: m.Status}
	}
	return matches, nil
}

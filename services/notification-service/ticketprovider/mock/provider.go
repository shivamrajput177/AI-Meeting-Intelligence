// Package mock implements ticketprovider.Provider against this service's
// own mock Jira board (notification.mock_jira_issues) — see
// docs/architecture/deployment-demo-strategy.md §3: "used by the public
// demo org, and it's genuinely bidirectional for free — an action item
// status change and a board-card drag are the same event on the same
// app, no webhook needed."
package mock

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider"
)

const providerName = "mock_jira"

type Provider struct {
	repo         repository.JiraRepository
	boardBaseURL string
}

// New builds a mock provider — boardBaseURL is the public API base (e.g.
// "http://localhost:8000/api/v1") a created ticket's URL points into,
// since the mock board has no address of its own the way a real Jira
// Cloud site does.
func New(repo repository.JiraRepository, boardBaseURL string) *Provider {
	return &Provider{repo: repo, boardBaseURL: boardBaseURL}
}

func (p *Provider) CreateTicket(ctx context.Context, orgID, actionItemID, title string) (ticketprovider.TicketRef, error) {
	issueKey, err := p.repo.CreateMockIssue(ctx, orgID, actionItemID, title)
	if err != nil {
		return ticketprovider.TicketRef{}, err
	}
	return ticketprovider.TicketRef{
		Provider: providerName,
		Key:      issueKey,
		URL:      p.boardBaseURL + "/orgs/" + orgID + "/mock-jira/board",
	}, nil
}

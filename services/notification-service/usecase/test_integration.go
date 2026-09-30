package usecase

import (
	"context"
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/email"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/orgs"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/slack"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// TestIntegrationUseCase serves POST /orgs/{orgId}/integrations/test (see
// docs/architecture/api-spec.md §Integrations: "Fire a test
// Slack/email/Jira call"). Unlike DispatchUseCase's transactional outbox,
// this fires the real external call synchronously and returns its
// outcome directly — the whole point of an owner/admin "test my
// integration" button is immediate pass/fail feedback, not a durably
// retried background send.
type TestIntegrationUseCase struct {
	orgs                   orgs.Client
	slack                  slack.Sender
	email                  email.Sender
	ticketProviders        map[string]ticketprovider.Provider
	defaultSlackWebhookURL string
	log                    *logger.Logger
}

func NewTestIntegrationUseCase(
	orgsClient orgs.Client, slackSender slack.Sender, emailSender email.Sender,
	ticketProviders map[string]ticketprovider.Provider, defaultSlackWebhookURL string, log *logger.Logger,
) *TestIntegrationUseCase {
	return &TestIntegrationUseCase{orgsClient, slackSender, emailSender, ticketProviders, defaultSlackWebhookURL, log}
}

// TestIntegration fires one real call on channel, resolved through the
// same per-org config (resolveSlackWebhookURL/resolveTicketProvider) that
// DispatchUseCase itself uses, so a passing test genuinely reflects what
// a real dispatch would do.
func (uc *TestIntegrationUseCase) TestIntegration(ctx context.Context, orgID, channel string, to *string) error {
	switch channel {
	case entity.ChannelSlack:
		webhookURL := resolveSlackWebhookURL(ctx, uc.orgs, orgID, uc.defaultSlackWebhookURL, uc.log)
		if webhookURL == "" {
			return apperr.BadRequest("no slack webhook configured for this organization")
		}
		return uc.slack.Send(ctx, webhookURL, "This is a test notification from AI Meeting Intelligence.")
	case entity.ChannelEmail:
		if to == nil || *to == "" {
			return apperr.BadRequest(`"to" is required to test the email integration`)
		}
		return uc.email.Send(ctx, *to, "AI Meeting Intelligence test notification",
			"This is a test notification from AI Meeting Intelligence.")
	case entity.ChannelJira:
		provider, err := resolveTicketProvider(ctx, uc.orgs, orgID, uc.ticketProviders)
		if err != nil {
			return apperr.BadRequest(err.Error())
		}
		if _, err := provider.CreateTicket(ctx, orgID, "integration-test", "Test ticket from AI Meeting Intelligence"); err != nil {
			return fmt.Errorf("create test ticket: %w", err)
		}
		return nil
	default:
		return apperr.BadRequest(fmt.Sprintf("unknown channel %q", channel))
	}
}

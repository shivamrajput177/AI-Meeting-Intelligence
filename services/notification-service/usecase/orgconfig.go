package usecase

import (
	"context"
	"fmt"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/orgs"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// resolveSlackWebhookURL resolves orgID's own configured Slack webhook via
// orgsClient, falling back to defaultURL when the org hasn't set one (or
// the lookup itself fails — sending to the known default is safer than
// blocking on an Organization Service blip). Shared by DispatchUseCase
// and TestIntegrationUseCase, which both need exactly this same per-org
// override semantics.
func resolveSlackWebhookURL(ctx context.Context, orgsClient orgs.Client, orgID, defaultURL string, log *logger.Logger) string {
	config, err := orgsClient.GetIntegrationConfig(ctx, orgID)
	if err != nil {
		log.Error("get org integration config, falling back to default slack webhook", "org_id", orgID, "err", err)
		return defaultURL
	}
	if config.SlackWebhookURL != nil && *config.SlackWebhookURL != "" {
		return *config.SlackWebhookURL
	}
	return defaultURL
}

// resolveTicketProvider resolves orgID's configured ticket provider (see
// entity.ValidTicketProviders — org.integration_configs.ticket_provider,
// defaulted to "mock_jira" for every org) to a concrete
// ticketprovider.Provider. Unlike resolveSlackWebhookURL, a lookup
// failure or an unregistered provider (as of this phase, that's any org
// configured for "atlassian_jira" — Phase 4.3's still-unbuilt stretch
// job) is returned as an error rather than silently guessing, since
// sending a ticket to the wrong provider can't be undone the way a
// re-sent Slack message can. Shared by DispatchUseCase and
// TestIntegrationUseCase.
func resolveTicketProvider(ctx context.Context, orgsClient orgs.Client, orgID string, providers map[string]ticketprovider.Provider) (ticketprovider.Provider, error) {
	config, err := orgsClient.GetIntegrationConfig(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("get org integration config: %w", err)
	}
	provider, ok := providers[config.TicketProvider]
	if !ok {
		return nil, fmt.Errorf("ticket provider %q not implemented yet", config.TicketProvider)
	}
	return provider, nil
}

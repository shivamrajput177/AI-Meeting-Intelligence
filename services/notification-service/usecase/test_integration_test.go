package usecase

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/orgs"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func TestTestIntegrationUseCase_Slack_UsesOrgWebhook(t *testing.T) {
	orgWebhook := "https://hooks.slack.example/org-specific"
	slackSender := &fakeSlackSender{}
	uc := NewTestIntegrationUseCase(
		&fakeOrgsClient{config: &orgs.IntegrationConfig{SlackWebhookURL: &orgWebhook, TicketProvider: "mock_jira"}},
		slackSender, &fakeEmailSender{}, map[string]ticketprovider.Provider{}, "https://hooks.slack.example/dev-default",
		logger.New("test", logger.LevelError),
	)

	if err := uc.TestIntegration(context.Background(), "org-1", entity.ChannelSlack, nil); err != nil {
		t.Fatalf("TestIntegration: %v", err)
	}
	if len(slackSender.sent) != 1 || slackSender.sent[0].webhookURL != orgWebhook {
		t.Fatalf("expected a test message sent to the org's own webhook, got %+v", slackSender.sent)
	}
}

func TestTestIntegrationUseCase_Slack_NoWebhookConfiguredIsBadRequest(t *testing.T) {
	uc := NewTestIntegrationUseCase(
		&fakeOrgsClient{}, &fakeSlackSender{}, &fakeEmailSender{}, map[string]ticketprovider.Provider{}, "",
		logger.New("test", logger.LevelError),
	)

	if err := uc.TestIntegration(context.Background(), "org-1", entity.ChannelSlack, nil); err == nil {
		t.Fatal("expected an error when no webhook is configured anywhere")
	}
}

func TestTestIntegrationUseCase_Email_RequiresTo(t *testing.T) {
	uc := NewTestIntegrationUseCase(
		&fakeOrgsClient{}, &fakeSlackSender{}, &fakeEmailSender{}, map[string]ticketprovider.Provider{}, "",
		logger.New("test", logger.LevelError),
	)

	if err := uc.TestIntegration(context.Background(), "org-1", entity.ChannelEmail, nil); err == nil {
		t.Fatal("expected an error when \"to\" is missing for an email test")
	}
}

func TestTestIntegrationUseCase_Email_SendsToRequestedAddress(t *testing.T) {
	emailSender := &fakeEmailSender{}
	uc := NewTestIntegrationUseCase(
		&fakeOrgsClient{}, &fakeSlackSender{}, emailSender, map[string]ticketprovider.Provider{}, "",
		logger.New("test", logger.LevelError),
	)

	to := "owner@example.com"
	if err := uc.TestIntegration(context.Background(), "org-1", entity.ChannelEmail, &to); err != nil {
		t.Fatalf("TestIntegration: %v", err)
	}
	if len(emailSender.sent) != 1 || emailSender.sent[0].to != to {
		t.Fatalf("expected a test email sent to %q, got %+v", to, emailSender.sent)
	}
}

func TestTestIntegrationUseCase_Jira_CreatesTestTicketViaConfiguredProvider(t *testing.T) {
	ticketProvider := &fakeTicketProvider{}
	uc := NewTestIntegrationUseCase(
		&fakeOrgsClient{config: &orgs.IntegrationConfig{TicketProvider: "mock_jira"}},
		&fakeSlackSender{}, &fakeEmailSender{}, map[string]ticketprovider.Provider{"mock_jira": ticketProvider}, "",
		logger.New("test", logger.LevelError),
	)

	if err := uc.TestIntegration(context.Background(), "org-1", entity.ChannelJira, nil); err != nil {
		t.Fatalf("TestIntegration: %v", err)
	}
}

func TestTestIntegrationUseCase_Jira_UnregisteredProviderIsBadRequest(t *testing.T) {
	uc := NewTestIntegrationUseCase(
		&fakeOrgsClient{config: &orgs.IntegrationConfig{TicketProvider: "atlassian_jira"}},
		&fakeSlackSender{}, &fakeEmailSender{}, map[string]ticketprovider.Provider{}, "",
		logger.New("test", logger.LevelError),
	)

	if err := uc.TestIntegration(context.Background(), "org-1", entity.ChannelJira, nil); err == nil {
		t.Fatal("expected an error for an unregistered ticket provider")
	}
}

func TestTestIntegrationUseCase_UnknownChannelIsBadRequest(t *testing.T) {
	uc := NewTestIntegrationUseCase(
		&fakeOrgsClient{}, &fakeSlackSender{}, &fakeEmailSender{}, map[string]ticketprovider.Provider{}, "",
		logger.New("test", logger.LevelError),
	)

	if err := uc.TestIntegration(context.Background(), "org-1", "carrier-pigeon", nil); err == nil {
		t.Fatal("expected an error for an unknown channel")
	}
}

package usecase_test

import (
	"context"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

// fakeIntegrationConfigRepository is an in-memory stand-in for
// *postgres.IntegrationConfigRepository — UpdateIntegrationConfig mimics
// the real COALESCE($n, column) semantics (a nil input field leaves the
// stored value unchanged) so these tests actually exercise the same
// partial-update contract the real repository provides.
type fakeIntegrationConfigRepository struct {
	configs map[string]*entity.IntegrationConfig
}

func newFakeIntegrationConfigRepository() *fakeIntegrationConfigRepository {
	return &fakeIntegrationConfigRepository{configs: map[string]*entity.IntegrationConfig{
		"org-1": {OrgID: "org-1", TicketProvider: entity.TicketProviderMockJira},
	}}
}

func (f *fakeIntegrationConfigRepository) GetIntegrationConfig(_ context.Context, orgID string) (*entity.IntegrationConfig, error) {
	config, ok := f.configs[orgID]
	if !ok {
		return nil, apperr.NotFound("integration config not found")
	}
	return config, nil
}

func (f *fakeIntegrationConfigRepository) UpdateIntegrationConfig(_ context.Context, orgID string, input entity.UpdateIntegrationConfigInput) (*entity.IntegrationConfig, error) {
	config, ok := f.configs[orgID]
	if !ok {
		return nil, apperr.NotFound("integration config not found")
	}
	if input.SlackWebhookURL != nil {
		config.SlackWebhookURL = input.SlackWebhookURL
	}
	if input.TicketProvider != nil {
		config.TicketProvider = *input.TicketProvider
	}
	if input.JiraBaseURL != nil {
		config.JiraBaseURL = input.JiraBaseURL
	}
	if input.JiraProjectKey != nil {
		config.JiraProjectKey = input.JiraProjectKey
	}
	if input.JiraAPITokenSecretRef != nil {
		config.JiraAPITokenSecretRef = input.JiraAPITokenSecretRef
	}
	return config, nil
}

func TestUpdateIntegrationConfigUseCase_RejectsInvalidTicketProvider(t *testing.T) {
	uc := usecase.NewUpdateIntegrationConfigUseCase(newFakeIntegrationConfigRepository())

	invalid := "carrier-pigeon-jira"
	_, err := uc.UpdateIntegrationConfig(context.Background(), "org-1", entity.UpdateIntegrationConfigInput{TicketProvider: &invalid})
	if err == nil {
		t.Fatal("expected an error for an invalid ticketProvider")
	}
}

func TestUpdateIntegrationConfigUseCase_AcceptsValidTicketProvider(t *testing.T) {
	uc := usecase.NewUpdateIntegrationConfigUseCase(newFakeIntegrationConfigRepository())

	atlassian := entity.TicketProviderAtlassianJira
	config, err := uc.UpdateIntegrationConfig(context.Background(), "org-1", entity.UpdateIntegrationConfigInput{TicketProvider: &atlassian})
	if err != nil {
		t.Fatalf("UpdateIntegrationConfig: %v", err)
	}
	if config.TicketProvider != entity.TicketProviderAtlassianJira {
		t.Fatalf("TicketProvider = %q, want %q", config.TicketProvider, entity.TicketProviderAtlassianJira)
	}
}

func TestUpdateIntegrationConfigUseCase_PartialUpdateLeavesOtherFieldsUnchanged(t *testing.T) {
	repo := newFakeIntegrationConfigRepository()
	uc := usecase.NewUpdateIntegrationConfigUseCase(repo)

	webhook := "https://hooks.slack.example/first"
	if _, err := uc.UpdateIntegrationConfig(context.Background(), "org-1", entity.UpdateIntegrationConfigInput{SlackWebhookURL: &webhook}); err != nil {
		t.Fatalf("UpdateIntegrationConfig (set webhook): %v", err)
	}

	projectKey := "DEMO"
	config, err := uc.UpdateIntegrationConfig(context.Background(), "org-1", entity.UpdateIntegrationConfigInput{JiraProjectKey: &projectKey})
	if err != nil {
		t.Fatalf("UpdateIntegrationConfig (set project key): %v", err)
	}
	if config.SlackWebhookURL == nil || *config.SlackWebhookURL != webhook {
		t.Fatalf("expected the earlier webhook update to survive an unrelated field's update, got %+v", config.SlackWebhookURL)
	}
	if config.JiraProjectKey == nil || *config.JiraProjectKey != projectKey {
		t.Fatalf("JiraProjectKey = %v, want %q", config.JiraProjectKey, projectKey)
	}
}

func TestGetIntegrationConfigUseCase_GetIntegrationConfig(t *testing.T) {
	uc := usecase.NewGetIntegrationConfigUseCase(newFakeIntegrationConfigRepository())

	config, err := uc.GetIntegrationConfig(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("GetIntegrationConfig: %v", err)
	}
	if config.TicketProvider != entity.TicketProviderMockJira {
		t.Fatalf("TicketProvider = %q, want %q", config.TicketProvider, entity.TicketProviderMockJira)
	}
}

func TestGetIntegrationConfigUseCase_GetIntegrationConfig_NotFound(t *testing.T) {
	uc := usecase.NewGetIntegrationConfigUseCase(newFakeIntegrationConfigRepository())

	if _, err := uc.GetIntegrationConfig(context.Background(), "does-not-exist"); err == nil {
		t.Fatal("expected a not-found error for an unknown org id")
	}
}

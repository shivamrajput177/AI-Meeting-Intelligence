package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/orgs"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/ticketprovider"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func marshalSlack(t *testing.T, text string) []byte {
	t.Helper()
	b, err := json.Marshal(entity.SlackPayload{Text: text})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func marshalEmail(t *testing.T, to, subject, body string) []byte {
	t.Helper()
	b, err := json.Marshal(entity.EmailPayload{To: to, Subject: subject, Body: body})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func marshalJira(t *testing.T, actionItemID, title string) []byte {
	t.Helper()
	b, err := json.Marshal(entity.JiraPayload{ActionItemID: actionItemID, Title: title})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// dispatchDeps bundles every fake DispatchUseCase depends on, so each test
// only needs to override the ones it cares about. ticketProvider is
// registered under the "mock_jira" key (see newDispatchUseCase) — orgs
// defaults to a fakeOrgsClient selecting that same provider with no Slack
// webhook override when left nil, so existing tests that don't care about
// per-org config keep working unchanged.
type dispatchDeps struct {
	repo           *fakeRepository
	jiraRepo       *fakeJiraRepository
	slack          *fakeSlackSender
	email          *fakeEmailSender
	ticketProvider *fakeTicketProvider
	actionItems    *fakeActionItemsClient
	orgs           *fakeOrgsClient
	pub            *fakePublisher
}

func newDispatchUseCase(d dispatchDeps, slackWebhookURL string) *DispatchUseCase {
	providers := map[string]ticketprovider.Provider{}
	if d.ticketProvider != nil {
		providers["mock_jira"] = d.ticketProvider
	}
	orgsClient := d.orgs
	if orgsClient == nil {
		orgsClient = &fakeOrgsClient{}
	}
	return NewDispatchUseCase(d.repo, d.jiraRepo, d.slack, d.email, providers, d.actionItems, orgsClient, d.pub, logger.New("test", logger.LevelError), slackWebhookURL)
}

func TestDispatchBatch_SlackSuccess(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-1", OrgID: "org-1", Channel: entity.ChannelSlack, Payload: marshalSlack(t, "hello")},
	}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "https://hooks.slack.example/webhook")

	n, err := uc.DispatchBatch(context.Background(), 20)
	if err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 dispatched, got %d", n)
	}
	if len(d.slack.sent) != 1 || d.slack.sent[0].text != "hello" || d.slack.sent[0].webhookURL != "https://hooks.slack.example/webhook" {
		t.Fatalf("unexpected slack sends: %+v", d.slack.sent)
	}
	if len(repo.sent) != 1 || repo.sent[0] != "row-1" {
		t.Fatalf("expected row-1 marked sent, got %v", repo.sent)
	}
	if len(d.pub.sent) != 1 || d.pub.sent[0].OutboxID != "row-1" {
		t.Fatalf("expected notification.sent.v1 published for row-1, got %+v", d.pub.sent)
	}
}

func TestDispatchBatch_EmailSuccess(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-2", OrgID: "org-1", Channel: entity.ChannelEmail, Payload: marshalEmail(t, "a@b.com", "subj", "body")},
	}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(d.email.sent) != 1 || d.email.sent[0].to != "a@b.com" {
		t.Fatalf("unexpected email sends: %+v", d.email.sent)
	}
}

func TestDispatchBatch_JiraSuccess(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-j1", OrgID: "org-1", Channel: entity.ChannelJira, Payload: marshalJira(t, "item-1", "Ship the API")},
	}}
	ticketProvider := &fakeTicketProvider{ref: ticketprovider.TicketRef{Provider: "mock_jira", Key: "DEMO-1", URL: "http://x/orgs/org-1/mock-jira/board"}}
	actionItems := &fakeActionItemsClient{}
	jiraRepo := &fakeJiraRepository{}
	d := dispatchDeps{repo: repo, jiraRepo: jiraRepo, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: ticketProvider, actionItems: actionItems, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(jiraRepo.links) != 1 || jiraRepo.links[0].issueKey != "DEMO-1" || jiraRepo.links[0].actionItemID != "item-1" {
		t.Fatalf("expected a jira_links upsert for DEMO-1/item-1, got %+v", jiraRepo.links)
	}
	if len(actionItems.updates) != 1 || actionItems.updates[0].jiraKey == nil || *actionItems.updates[0].jiraKey != "DEMO-1" {
		t.Fatalf("expected the action item's jira issue key written back, got %+v", actionItems.updates)
	}
	if len(repo.sent) != 1 || repo.sent[0] != "row-j1" {
		t.Fatalf("expected row-j1 marked sent, got %v", repo.sent)
	}
}

func TestDispatchBatch_JiraWriteBackFailureDoesNotFailDispatch(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-j2", OrgID: "org-1", Channel: entity.ChannelJira, Payload: marshalJira(t, "item-2", "Fix the bug")},
	}}
	ticketProvider := &fakeTicketProvider{ref: ticketprovider.TicketRef{Provider: "mock_jira", Key: "DEMO-2"}}
	actionItems := &fakeActionItemsClient{err: errFake}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: ticketProvider, actionItems: actionItems, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	// The ticket itself was created successfully, so the row is still
	// marked sent even though the write-back onto the action item failed
	// — see dispatch.go's own doc comment on why that's best-effort.
	if len(repo.sent) != 1 || repo.sent[0] != "row-j2" {
		t.Fatalf("expected row-j2 still marked sent despite the write-back failure, got %v", repo.sent)
	}
}

func TestDispatchBatch_RetriesOnFailure(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-3", OrgID: "org-1", Channel: entity.ChannelSlack, Attempts: 1, Payload: marshalSlack(t, "hello")},
	}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{err: errFake}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "url")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(repo.failed) != 1 || repo.failed[0] != "row-3" {
		t.Fatalf("expected row-3 recorded as a failed attempt, got %v", repo.failed)
	}
	if len(d.pub.failed) != 0 {
		t.Fatalf("expected no notification.failed.v1 yet (attempts below MaxDispatchAttempts), got %+v", d.pub.failed)
	}
}

func TestDispatchBatch_GivesUpAfterMaxAttempts(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-4", OrgID: "org-1", Channel: entity.ChannelSlack, Attempts: MaxDispatchAttempts - 1, Payload: marshalSlack(t, "hello")},
	}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{err: errFake}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "url")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(d.pub.failed) != 1 || d.pub.failed[0].OutboxID != "row-4" {
		t.Fatalf("expected notification.failed.v1 published for row-4, got %+v", d.pub.failed)
	}
}

// TestDispatchBatch_SlackUsesOrgWebhookOverDefault covers Phase 4.5's
// per-org Slack webhook override: an org with its own configured webhook
// (via orgs.Client) takes precedence over this service's dev-config-wide
// default, even though a default was also supplied.
func TestDispatchBatch_SlackUsesOrgWebhookOverDefault(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-org-webhook", OrgID: "org-1", Channel: entity.ChannelSlack, Payload: marshalSlack(t, "hello")},
	}}
	orgWebhook := "https://hooks.slack.example/org-specific"
	orgsClient := &fakeOrgsClient{config: &orgs.IntegrationConfig{SlackWebhookURL: &orgWebhook, TicketProvider: "mock_jira"}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, orgs: orgsClient, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "https://hooks.slack.example/dev-config-default")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(d.slack.sent) != 1 || d.slack.sent[0].webhookURL != orgWebhook {
		t.Fatalf("expected the org's own webhook to be used, got %+v", d.slack.sent)
	}
}

// TestDispatchBatch_SlackFallsBackToDefaultWhenOrgUnset covers the other
// half: an org with no Slack webhook configured of its own falls back to
// this service's dev-config-wide default.
func TestDispatchBatch_SlackFallsBackToDefaultWhenOrgUnset(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-fallback", OrgID: "org-1", Channel: entity.ChannelSlack, Payload: marshalSlack(t, "hello")},
	}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, orgs: &fakeOrgsClient{}, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "https://hooks.slack.example/dev-config-default")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(d.slack.sent) != 1 || d.slack.sent[0].webhookURL != "https://hooks.slack.example/dev-config-default" {
		t.Fatalf("expected the dev-config default webhook, got %+v", d.slack.sent)
	}
}

// TestDispatchBatch_JiraUnregisteredProviderFails covers Phase 4.5's
// ticket-provider registry: an org configured for a provider with no
// entry in ticketProviders (e.g. "atlassian_jira", still unbuilt) fails
// the dispatch with a clear error rather than silently using whatever
// provider happens to be registered.
func TestDispatchBatch_JiraUnregisteredProviderFails(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-unregistered", OrgID: "org-1", Channel: entity.ChannelJira, Payload: marshalJira(t, "item-1", "Ship the API")},
	}}
	orgsClient := &fakeOrgsClient{config: &orgs.IntegrationConfig{TicketProvider: "atlassian_jira"}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, orgs: orgsClient, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(repo.failed) != 1 || repo.failed[0] != "row-unregistered" {
		t.Fatalf("expected the row recorded as a failed attempt, got %v", repo.failed)
	}
	if len(repo.sent) != 0 {
		t.Fatalf("expected no successful send for an unregistered provider, got %v", repo.sent)
	}
}

func TestDispatchBatch_UnknownChannel(t *testing.T) {
	repo := &fakeRepository{claimRows: []entity.OutboxRow{
		{ID: "row-5", OrgID: "org-1", Channel: "carrier-pigeon", Payload: []byte("{}")},
	}}
	d := dispatchDeps{repo: repo, jiraRepo: &fakeJiraRepository{}, slack: &fakeSlackSender{}, email: &fakeEmailSender{}, ticketProvider: &fakeTicketProvider{}, actionItems: &fakeActionItemsClient{}, pub: &fakePublisher{}}
	uc := newDispatchUseCase(d, "")

	if _, err := uc.DispatchBatch(context.Background(), 20); err != nil {
		t.Fatalf("DispatchBatch: %v", err)
	}
	if len(repo.failed) != 1 {
		t.Fatalf("expected the unknown-channel row recorded as a failed attempt, got %v", repo.failed)
	}
}

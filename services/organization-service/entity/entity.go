// Package entity holds Organization Service's plain data structs: its
// own internal model (Organization — what repository reads out of
// Postgres and usecase operates on), the JSON wire shapes at this
// service's REST boundary, and the plain input struct its usecase layer
// passes around internally. Nothing here has behavior (no methods, just
// fields, and json tags where the struct crosses the wire): it's data,
// not a class, which is what keeps it out of handler/ and usecase/ —
// those packages hold the code that does something with an entity, this
// package only describes its shape.
package entity

import "time"

// Organization is this service's own internal model — what repository
// reads out of Postgres and usecase operates on. Not JSON-tagged: it
// never crosses the wire directly, handler.go always reshapes it into
// an OrgResponse first (see toOrgResponse).
type Organization struct {
	ID        string
	Name      string
	Slug      string
	Plan      string
	Status    string
	CreatedAt time.Time
}

// CreateOrgRequest is the body of both the internal POST /internal/orgs
// call (from Auth Service during signup) and, in a later phase, any
// public "create a second org" flow.
type CreateOrgRequest struct {
	Name string `json:"name"`
}

type OrgResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// CreateOrgInput is usecase.CreateOrgUseCase.CreateOrg's input — the
// usecase layer's own Go-to-Go call contract, not a JSON wire struct
// (no json tags), passed by handler/ straight from a decoded
// CreateOrgRequest.
type CreateOrgInput struct {
	Name string
}

// Ticket provider values — the CHECK constraint on
// org.integration_configs.ticket_provider (see migrations/0002's own doc
// comment on why this column exists at all). Only TicketProviderMockJira
// has a real dispatch implementation anywhere in this codebase —
// TicketProviderAtlassianJira is accepted and stored (Phase 4.3's real
// Jira provider is an explicit, still-skipped stretch item), and
// Notification Service's own dispatch returns a clear "not implemented"
// error if an org is ever configured with it, rather than silently
// falling back to mock.
const (
	TicketProviderMockJira      = "mock_jira"
	TicketProviderAtlassianJira = "atlassian_jira"
)

var ValidTicketProviders = map[string]bool{TicketProviderMockJira: true, TicketProviderAtlassianJira: true}

// IntegrationConfig is this service's own internal model of one
// org.integration_configs row — what repository reads/writes and usecase
// operates on. Not JSON-tagged: handler.go always reshapes it into an
// IntegrationConfigResponse first. Every org gets one of these at
// creation time (see OrgRepository.Create's own doc comment), so this is
// never nil for an org that exists — no separate "not configured yet"
// state to handle.
type IntegrationConfig struct {
	OrgID                 string
	SlackWebhookURL       *string
	TicketProvider        string
	JiraBaseURL           *string
	JiraProjectKey        *string
	JiraAPITokenSecretRef *string
	SMTPConfigSecretRef   *string
}

// --- handler.go: PATCH /orgs/{orgId}/settings' own REST API. Scoped to
// integration config only (docs/ROADMAP.md Phase 4.5's actual task,
// "Org-level integration config CRUD") — general org settings (name,
// plan) have no usecase built for them in any phase yet, so this route
// doesn't invent one speculatively. SMTPConfigSecretRef is deliberately
// not exposed here: nothing in this codebase reads it yet (Notification
// Service's SMTP settings are still dev-config-wide, not per-org — see
// README's own honest note on that gap), so there's no real value yet
// in letting a caller set it. ---

type UpdateIntegrationConfigRequest struct {
	SlackWebhookURL       *string `json:"slackWebhookUrl,omitempty"`
	TicketProvider        *string `json:"ticketProvider,omitempty"`
	JiraBaseURL           *string `json:"jiraBaseUrl,omitempty"`
	JiraProjectKey        *string `json:"jiraProjectKey,omitempty"`
	JiraAPITokenSecretRef *string `json:"jiraApiTokenSecretRef,omitempty"`
}

type IntegrationConfigResponse struct {
	SlackWebhookURL       *string `json:"slackWebhookUrl,omitempty"`
	TicketProvider        string  `json:"ticketProvider"`
	JiraBaseURL           *string `json:"jiraBaseUrl,omitempty"`
	JiraProjectKey        *string `json:"jiraProjectKey,omitempty"`
	JiraAPITokenSecretRef *string `json:"jiraApiTokenSecretRef,omitempty"`
}

// UpdateIntegrationConfigInput is UpdateIntegrationConfigUseCase.Execute's
// input — the usecase layer's own Go-to-Go call contract (no json tags),
// passed by handler/ straight from a decoded UpdateIntegrationConfigRequest.
type UpdateIntegrationConfigInput struct {
	SlackWebhookURL       *string
	TicketProvider        *string
	JiraBaseURL           *string
	JiraProjectKey        *string
	JiraAPITokenSecretRef *string
}

// integration_config_repo.go implements
// repository.IntegrationConfigRepository against org.integration_configs
// — see entity.IntegrationConfig's own doc comment. Kept as its own file
// alongside org_repo.go's OrgRepository (same package) since organizations
// and integration config are genuinely distinct data-access concerns —
// see repository/interface.go's own doc comment on that split.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/organization-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type IntegrationConfigRepository struct {
	pool *pgxpool.Pool
}

func NewIntegrationConfigRepository(pool *pgxpool.Pool) *IntegrationConfigRepository {
	return &IntegrationConfigRepository{pool: pool}
}

const integrationConfigCols = `org_id, slack_webhook_url, ticket_provider, jira_base_url, jira_project_key, jira_api_token_secret_ref, smtp_config_secret_ref`

func (r *IntegrationConfigRepository) GetIntegrationConfig(ctx context.Context, orgID string) (*entity.IntegrationConfig, error) {
	var c entity.IntegrationConfig
	err := r.pool.QueryRow(ctx, `SELECT `+integrationConfigCols+` FROM org.integration_configs WHERE org_id = $1`, orgID).Scan(
		&c.OrgID, &c.SlackWebhookURL, &c.TicketProvider, &c.JiraBaseURL, &c.JiraProjectKey, &c.JiraAPITokenSecretRef, &c.SMTPConfigSecretRef,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("integration config not found")
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateIntegrationConfig assumes the row already exists (every org gets
// one at creation — see OrgRepository.Create's own doc comment), so this
// is always a plain UPDATE, never an upsert.
func (r *IntegrationConfigRepository) UpdateIntegrationConfig(ctx context.Context, orgID string, input entity.UpdateIntegrationConfigInput) (*entity.IntegrationConfig, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE org.integration_configs SET
			slack_webhook_url = COALESCE($1, slack_webhook_url),
			ticket_provider = COALESCE($2, ticket_provider),
			jira_base_url = COALESCE($3, jira_base_url),
			jira_project_key = COALESCE($4, jira_project_key),
			jira_api_token_secret_ref = COALESCE($5, jira_api_token_secret_ref)
		WHERE org_id = $6
	`, input.SlackWebhookURL, input.TicketProvider, input.JiraBaseURL, input.JiraProjectKey, input.JiraAPITokenSecretRef, orgID)
	if err != nil {
		return nil, err
	}
	return r.GetIntegrationConfig(ctx, orgID)
}

// jira_repo.go implements repository.JiraRepository against the
// notification.jira_links / notification.mock_jira_issues tables — see
// docs/architecture/deployment-demo-strategy.md §3. Kept as its own file
// alongside outbox_repo.go's OutboxRepository (same package, same doc
// comment on the org_id filtering / RLS rationale up there) since Outbox
// and Jira are genuinely distinct data-access concerns — see
// repository/interface.go's own doc comment on that split.
//
// CreateMockIssue's per-org sequential issue-key numbering
// (SELECT MAX(...) + 1 in a subquery, not a dedicated sequence or
// advisory lock) has a real, accepted race: two concurrent ticket
// creations for the same org could compute the same next number. The
// table's UNIQUE (org_id, issue_key) index turns that into a constraint
// violation rather than a silent duplicate, which DispatchUseCase's
// existing bounded-retry-then-give-up loop already retries — an
// acceptable trade-off for a demo-scale mock board, not a real ticketing
// system's guarantee.
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/apperr"
)

type JiraRepository struct {
	pool *pgxpool.Pool
}

func NewJiraRepository(pool *pgxpool.Pool) *JiraRepository {
	return &JiraRepository{pool: pool}
}

func (r *JiraRepository) CreateMockIssue(ctx context.Context, orgID, actionItemID, title string) (string, error) {
	var issueKey string
	err := r.pool.QueryRow(ctx, `
		INSERT INTO notification.mock_jira_issues (id, org_id, action_item_id, issue_key, project_key, title, status, created_at, updated_at)
		VALUES (
			gen_random_uuid(), $1, $2,
			'DEMO-' || (
				SELECT COALESCE(MAX(NULLIF(regexp_replace(issue_key, '^DEMO-', ''), '')::int), 0) + 1
				FROM notification.mock_jira_issues WHERE org_id = $1
			),
			'DEMO', $3, 'To Do', now(), now()
		)
		RETURNING issue_key
	`, orgID, actionItemID, title).Scan(&issueKey)
	return issueKey, err
}

func (r *JiraRepository) TransitionMockIssue(ctx context.Context, orgID, issueKey, newStatus string) (string, error) {
	var actionItemID string
	err := r.pool.QueryRow(ctx, `
		UPDATE notification.mock_jira_issues SET status = $1, updated_at = now()
		WHERE org_id = $2 AND issue_key = $3
		RETURNING action_item_id
	`, newStatus, orgID, issueKey).Scan(&actionItemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.NotFound("mock jira issue not found")
	}
	return actionItemID, err
}

func (r *JiraRepository) ListMockBoard(ctx context.Context, orgID string) ([]entity.MockJiraIssue, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, org_id, action_item_id, issue_key, project_key, title, status, created_at, updated_at
		FROM notification.mock_jira_issues WHERE org_id = $1 ORDER BY created_at
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []entity.MockJiraIssue
	for rows.Next() {
		var issue entity.MockJiraIssue
		if err := rows.Scan(&issue.ID, &issue.OrgID, &issue.ActionItemID, &issue.IssueKey,
			&issue.ProjectKey, &issue.Title, &issue.Status, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, issue)
	}
	return out, rows.Err()
}

func (r *JiraRepository) UpsertJiraLink(ctx context.Context, orgID, actionItemID, provider, issueKey, url string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification.jira_links (action_item_id, org_id, provider, jira_issue_key, jira_url, created_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (action_item_id) DO UPDATE SET
			provider = EXCLUDED.provider, jira_issue_key = EXCLUDED.jira_issue_key, jira_url = EXCLUDED.jira_url
	`, actionItemID, orgID, provider, issueKey, url)
	return err
}

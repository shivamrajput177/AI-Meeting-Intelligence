// Package actionitems defines the Client interface usecase depends on;
// actionitems/http, right below this package in the same tree, implements
// it against Action Item Service's internal REST API. Keeping the
// interface here instead of off in some unrelated package is just where
// it belongs — its one real implementation lives one directory down.
package actionitems

import "context"

type Client interface {
	// UpdateActionItem PATCHes an action item's status and/or Jira issue
	// key — either argument left nil leaves that field unchanged, per
	// actionitemsvc's own UpdateActionItemInternalRequest shape. Used
	// two ways: DispatchUseCase writes back JiraIssueKey after creating a
	// ticket, and TransitionMockIssueUseCase writes back Status after a
	// mock-board card drag (see deployment-demo-strategy.md §3's
	// "genuinely bidirectional" design).
	UpdateActionItem(ctx context.Context, orgID, actionItemID string, status, jiraIssueKey *string) error
}

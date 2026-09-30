// Package users defines the Client interface usecase depends on;
// users/http, right below this package in the same tree, implements it
// against User Service's internal REST API. Keeping the interface here
// instead of off in some unrelated package is just where it belongs — its
// one real implementation lives one directory down.
package users

import "context"

type Client interface {
	// GetEmail resolves userID's email — this service's own read for the
	// "summary ready" email notification, since neither
	// summary.completed.v1 nor meeting-service's meeting record carries an
	// email address directly, only a user id (meeting.created_by).
	GetEmail(ctx context.Context, orgID, userID string) (string, error)
}

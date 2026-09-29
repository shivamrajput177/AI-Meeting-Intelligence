// Package summary defines the Client interface usecase depends on;
// summary/http, right below this package in the same tree, implements it
// against AI Summary Service's internal REST API. Keeping the interface
// here instead of off in some unrelated package is just where it
// belongs — its one real implementation lives one directory down.
package summary

import "context"

// Summary is this service's own Go-to-Go shape for the three phrase lists
// a summary carries — the raw material for the topic-frequency rollup's
// keyword-proxy extraction (see usecase/record_topics.go).
type Summary struct {
	KeyDecisions []string
	Risks        []string
	Blockers     []string
}

type Client interface {
	// GetSummary returns meetingID's summary — this service's own read for
	// summary.completed.v1, which only carries a summary_id (see
	// entity.SummaryCompletedEvent), not the phrase lists the
	// topic-frequency rollup extracts topics from.
	GetSummary(ctx context.Context, orgID, meetingID string) (*Summary, error)
}

// Package chunks defines the Client interface usecase depends on;
// chunks/http, right below this package in the same tree, implements it
// against AI Summary Service's internal REST API — see that package's doc
// comment. Keeping the interface here instead of off in some unrelated
// package is just where it belongs — its one real implementation lives
// one directory down.
package chunks

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/search-service/entity"
)

type Client interface {
	ListChunks(ctx context.Context, orgID, meetingID string) ([]entity.Chunk, error)
}

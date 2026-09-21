// Package events defines the Publisher interface usecase (and
// consumer.go's retry loop) depend on; events/kafka, right below this
// package in the same tree, implements it against Kafka — see that
// package's doc comment. Keeping the interface here instead of off in
// some unrelated package is just where it belongs — its one real
// implementation lives one directory down.
package events

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/ai-summary-service/entity"
)

type Publisher interface {
	PublishChunkCreated(ctx context.Context, event entity.ChunkCreatedEvent) error
	PublishSummaryCompleted(ctx context.Context, event entity.SummaryCompletedEvent) error
	PublishSummaryFailed(ctx context.Context, event entity.SummaryFailedEvent) error
}

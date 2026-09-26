// Package events defines the Publisher interface usecase (and
// consumer.go's retry loop) depend on; events/kafka, right below this
// package in the same tree, implements it against Kafka — see that
// package's doc comment. Keeping the interface here instead of off in
// some unrelated package is just where it belongs — its one real
// implementation lives one directory down.
package events

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

type Publisher interface {
	PublishActionItemExtracted(ctx context.Context, event entity.ActionItemExtractedEvent) error
	PublishActionItemExtractionFailed(ctx context.Context, event entity.ActionItemExtractionFailedEvent) error
}

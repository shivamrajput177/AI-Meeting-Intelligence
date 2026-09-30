// Package events defines the Publisher interface usecase depends on;
// events/kafka, right below this package in the same tree, implements it
// against Kafka — see that package's doc comment. Keeping the interface
// here instead of off in some unrelated package is just where it
// belongs — its one real implementation lives one directory down.
package events

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

type Publisher interface {
	PublishNotificationSent(ctx context.Context, event entity.NotificationSentEvent) error
	PublishNotificationFailed(ctx context.Context, event entity.NotificationFailedEvent) error
}

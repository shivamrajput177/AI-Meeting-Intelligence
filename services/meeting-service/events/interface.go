// Package events defines the Publisher interface usecase depends on;
// events/kafka, right below this package in the same tree, implements it
// against Kafka — see that package's doc comment. Keeping the interface
// here instead of off in some unrelated package is just where it
// belongs — its one real implementation lives one directory down.
package events

import (
	"context"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/meeting-service/entity"
)

type Publisher interface {
	PublishMeetingUploaded(ctx context.Context, event entity.MeetingUploadedEvent) error
}

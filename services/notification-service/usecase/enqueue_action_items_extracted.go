package usecase

import (
	"context"
	"encoding/json"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
)

// EnqueueActionItemDigestUseCase turns action-item.extracted.v1 into a
// Slack outbox row — the "action items digest" half of
// docs/architecture/kafka-topics.md's flow-1 diagram.
type EnqueueActionItemDigestUseCase struct {
	meetings meetings.Client
	repo     repository.Repository
}

func NewEnqueueActionItemDigestUseCase(meetings meetings.Client, repo repository.Repository) *EnqueueActionItemDigestUseCase {
	return &EnqueueActionItemDigestUseCase{meetings, repo}
}

func (uc *EnqueueActionItemDigestUseCase) EnqueueActionItemDigest(ctx context.Context, orgID, meetingID string, itemCount int) error {
	meeting, err := uc.meetings.GetMeeting(ctx, orgID, meetingID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(entity.SlackPayload{Text: renderActionItemDigestSlackText(itemCount, meeting.Title)})
	if err != nil {
		return err
	}
	return uc.repo.Enqueue(ctx, orgID, entity.ChannelSlack, payload)
}

package usecase

import (
	"context"
	"encoding/json"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/meetings"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/users"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

// EnqueueSummaryNotificationsUseCase turns summary.completed.v1 into two
// outbox rows — see docs/architecture/kafka-topics.md's flow-1 diagram
// (`NO-->>U: Slack "meeting summarized" + action items digest`): a Slack
// message here, and an email to the meeting's creator (the outbox
// pattern's "durable write" half — see entity.OutboxRow's doc comment).
type EnqueueSummaryNotificationsUseCase struct {
	meetings meetings.Client
	users    users.Client
	repo     repository.Repository
	log      *logger.Logger
}

func NewEnqueueSummaryNotificationsUseCase(meetings meetings.Client, users users.Client, repo repository.Repository, log *logger.Logger) *EnqueueSummaryNotificationsUseCase {
	return &EnqueueSummaryNotificationsUseCase{meetings, users, repo, log}
}

// EnqueueSummaryNotifications always enqueues the Slack row (it needs only
// the meeting's title); the email row is best-effort — a user lookup
// failure (e.g. the creator was since deactivated) is logged and skipped
// rather than failing the whole call, since losing one email notification
// shouldn't also cost the Slack message that already succeeded.
func (uc *EnqueueSummaryNotificationsUseCase) EnqueueSummaryNotifications(ctx context.Context, orgID, meetingID string) error {
	meeting, err := uc.meetings.GetMeeting(ctx, orgID, meetingID)
	if err != nil {
		return err
	}

	slackPayload, err := json.Marshal(entity.SlackPayload{Text: renderSummaryCompletedSlackText(meeting.Title)})
	if err != nil {
		return err
	}
	if err := uc.repo.Enqueue(ctx, orgID, entity.ChannelSlack, slackPayload); err != nil {
		return err
	}

	email, err := uc.users.GetEmail(ctx, orgID, meeting.CreatedBy)
	if err != nil {
		uc.log.Error("resolve meeting creator's email, skipping email notification", "meeting_id", meetingID, "created_by", meeting.CreatedBy, "err", err)
		return nil
	}
	subject, body := renderSummaryReadyEmail(meeting.Title)
	emailPayload, err := json.Marshal(entity.EmailPayload{To: email, Subject: subject, Body: body})
	if err != nil {
		return err
	}
	return uc.repo.Enqueue(ctx, orgID, entity.ChannelEmail, emailPayload)
}

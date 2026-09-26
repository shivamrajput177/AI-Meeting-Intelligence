package usecase

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/events"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/llm"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/participants"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/repository"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/summary"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

type ExtractActionItemsUseCase struct {
	summaryClient      summary.Client
	participantsClient participants.Client
	extractor          llm.Extractor
	repo               repository.Repository
	publisher          events.Publisher
	log                *logger.Logger
}

func NewExtractActionItemsUseCase(
	summaryClient summary.Client,
	participantsClient participants.Client,
	extractor llm.Extractor,
	repo repository.Repository,
	publisher events.Publisher,
	log *logger.Logger,
) *ExtractActionItemsUseCase {
	return &ExtractActionItemsUseCase{
		summaryClient: summaryClient, participantsClient: participantsClient,
		extractor: extractor, repo: repo, publisher: publisher, log: log,
	}
}

// ExtractActionItems is summary.completed.v1's business logic (see
// consumer.go): fetch the meeting's summary and participants, run the
// action-item extraction prompt via Ollama, turn the summary's own
// keyDecisions/risks/blockers (already-extracted structured text — no
// second LLM call needed for those, see llm.Extractor's doc comment)
// into rows alongside the newly-extracted action items, best-guess an
// owner for each action item against the participant list, persist the
// whole batch, and publish action-item.extracted.v1. It deliberately
// doesn't publish a failure event itself — same reasoning as
// aisummarysvc's ProcessTranscriptUsecase: consumer.go's retry loop owns
// deciding when an error is worth retrying versus when to give up and
// publish action-item.extraction-failed.v1, so that decision lives in
// exactly one place.
func (uc *ExtractActionItemsUseCase) ExtractActionItems(ctx context.Context, orgID, meetingID string) ([]*entity.ActionItem, error) {
	s, err := uc.summaryClient.GetSummary(ctx, orgID, meetingID)
	if err != nil {
		return nil, fmt.Errorf("fetch summary: %w", err)
	}

	people, err := uc.participantsClient.ListParticipants(ctx, orgID, meetingID)
	if err != nil {
		return nil, fmt.Errorf("fetch participants: %w", err)
	}

	extracted, err := uc.extractor.ExtractActionItems(ctx, s.SummaryText)
	if err != nil {
		return nil, fmt.Errorf("ollama action-item extraction: %w", err)
	}

	var items []*entity.ActionItem
	for i := range extracted {
		e := extracted[i]
		var ownerRawName *string
		if e.OwnerRawName != "" {
			ownerRawName = &e.OwnerRawName
		}
		confidence := e.Confidence
		items = append(items, &entity.ActionItem{
			ID: uuid.NewString(), MeetingID: meetingID, OrgID: orgID,
			Description: e.Description, Type: entity.TypeAction,
			OwnerUserID: matchOwner(e.OwnerRawName, people), OwnerRawName: ownerRawName,
			DueDate: e.DueDate, Status: entity.StatusOpen, Priority: e.Priority,
			Confidence: &confidence,
		})
	}
	items = append(items, plainItems(meetingID, orgID, entity.TypeDecision, s.KeyDecisions)...)
	items = append(items, plainItems(meetingID, orgID, entity.TypeRisk, s.Risks)...)
	items = append(items, plainItems(meetingID, orgID, entity.TypeBlocker, s.Blockers)...)

	if err := uc.repo.ReplaceActionItems(ctx, orgID, meetingID, items); err != nil {
		return nil, fmt.Errorf("store action items: %w", err)
	}

	if err := uc.publisher.PublishActionItemExtracted(ctx, entity.ActionItemExtractedEvent{
		MeetingID: meetingID, OrgID: orgID, ItemCount: len(items),
	}); err != nil {
		// Best-effort, same trade-off as aisummarysvc's own
		// chunk.created.v1/summary.completed.v1 publishes: no
		// transactional outbox yet, so a dropped event here is a real,
		// logged gap rather than a silently swallowed one.
		uc.log.Error("publish action-item.extracted.v1 failed", "meeting_id", meetingID, "err", err)
	}
	return items, nil
}

// plainItems turns one of Summary's already-extracted string lists
// (KeyDecisions/Risks/Blockers) into unassigned actionitem rows of the
// given type — no owner, no due date, no LLM confidence score, since
// these were already pulled out as plain text by ai-summary-service's own
// summarization prompt, not guessed here.
func plainItems(meetingID, orgID, itemType string, descriptions []string) []*entity.ActionItem {
	items := make([]*entity.ActionItem, len(descriptions))
	for i, d := range descriptions {
		items[i] = &entity.ActionItem{
			ID: uuid.NewString(), MeetingID: meetingID, OrgID: orgID,
			Description: d, Type: itemType, Status: entity.StatusOpen, Priority: entity.PriorityMedium,
		}
	}
	return items
}

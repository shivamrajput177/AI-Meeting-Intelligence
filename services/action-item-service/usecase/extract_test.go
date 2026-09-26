package usecase_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/usecase"
	"github.com/shivamrajput177/ai-meeting-intelligence/shared/logger"
)

func strPtr(s string) *string { return &s }

// fakeSummaryClient, fakeParticipantsClient, fakeExtractor,
// fakeRepository, and fakePublisher are in-memory stand-ins for the real
// AI Summary Service HTTP client / Meeting Service HTTP client / Ollama
// client / Postgres repository / Kafka producer — this is the whole point
// of defining each as an interface usecase depends on (see
// docs/architecture/folder-structure.md's Clean Architecture layering):
// ExtractActionItemsUseCase is fully testable with none of those real
// systems involved.
type fakeSummaryClient struct {
	summary *entity.Summary
	err     error
}

func (f *fakeSummaryClient) GetSummary(_ context.Context, _, _ string) (*entity.Summary, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.summary, nil
}

type fakeParticipantsClient struct {
	people []entity.Participant
	err    error
}

func (f *fakeParticipantsClient) ListParticipants(_ context.Context, _, _ string) ([]entity.Participant, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.people, nil
}

type fakeExtractor struct {
	items []entity.ExtractedItem
	err   error
}

func (f *fakeExtractor) ExtractActionItems(_ context.Context, _ string) ([]entity.ExtractedItem, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

type fakeRepository struct {
	mu    sync.Mutex
	items []*entity.ActionItem
}

func (f *fakeRepository) ReplaceActionItems(_ context.Context, _, _ string, items []*entity.ActionItem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items = items
	return nil
}
func (f *fakeRepository) GetByID(_ context.Context, _, id string) (*entity.ActionItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, it := range f.items {
		if it.ID == id {
			return it, nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeRepository) ListByMeeting(_ context.Context, _, _ string) ([]*entity.ActionItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.items, nil
}
func (f *fakeRepository) List(_ context.Context, _ string, _ entity.ListActionItemsFilter) ([]*entity.ActionItem, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.items, len(f.items), nil
}
func (f *fakeRepository) Update(_ context.Context, _, _ string, _ entity.UpdateActionItemInput) (*entity.ActionItem, error) {
	return nil, errors.New("not implemented in fake")
}

type fakePublisher struct {
	mu        sync.Mutex
	extracted []entity.ActionItemExtractedEvent
	failed    []entity.ActionItemExtractionFailedEvent
}

func (f *fakePublisher) PublishActionItemExtracted(_ context.Context, event entity.ActionItemExtractedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.extracted = append(f.extracted, event)
	return nil
}
func (f *fakePublisher) PublishActionItemExtractionFailed(_ context.Context, event entity.ActionItemExtractionFailedEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failed = append(f.failed, event)
	return nil
}

func TestExtractActionItemsUseCase_ExtractActionItems(t *testing.T) {
	summaryClient := &fakeSummaryClient{summary: &entity.Summary{
		MeetingID:    "meeting-1",
		SummaryText:  "Shivam will update the docs by Friday.",
		KeyDecisions: []string{"Ship on Friday"},
		Risks:        []string{"Vendor API might be flaky"},
		Blockers:     []string{"Design review"},
	}}
	participantsClient := &fakeParticipantsClient{people: []entity.Participant{
		{UserID: strPtr("user-1"), Email: "shivam@example.com", DisplayName: "Shivam"},
	}}
	extractor := &fakeExtractor{items: []entity.ExtractedItem{
		{Description: "Update the docs", Type: entity.TypeAction, OwnerRawName: "Shivam", Priority: entity.PriorityMedium, Confidence: 0.9},
		{Description: "Unassigned follow-up", Type: entity.TypeAction, OwnerRawName: "", Priority: entity.PriorityLow, Confidence: 0.5},
	}}
	repo := &fakeRepository{}
	publisher := &fakePublisher{}

	uc := usecase.NewExtractActionItemsUseCase(summaryClient, participantsClient, extractor, repo, publisher, logger.New("test", logger.LevelError))
	items, err := uc.ExtractActionItems(context.Background(), "org-1", "meeting-1")
	if err != nil {
		t.Fatalf("ExtractActionItems: %v", err)
	}

	// 2 extracted action items + 1 decision + 1 risk + 1 blocker.
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d: %+v", len(items), items)
	}

	var matched, unassigned *entity.ActionItem
	var decisionCount, riskCount, blockerCount int
	for _, it := range items {
		switch {
		case it.Description == "Update the docs":
			matched = it
		case it.Description == "Unassigned follow-up":
			unassigned = it
		case it.Type == entity.TypeDecision:
			decisionCount++
		case it.Type == entity.TypeRisk:
			riskCount++
		case it.Type == entity.TypeBlocker:
			blockerCount++
		}
	}
	if matched == nil || matched.OwnerUserID == nil || *matched.OwnerUserID != "user-1" {
		t.Fatalf("expected the extracted item naming Shivam to match user-1, got %+v", matched)
	}
	if unassigned == nil || unassigned.OwnerUserID != nil {
		t.Fatalf("expected the unnamed extracted item to have no owner, got %+v", unassigned)
	}
	if decisionCount != 1 || riskCount != 1 || blockerCount != 1 {
		t.Fatalf("expected 1 decision/risk/blocker each, got %d/%d/%d", decisionCount, riskCount, blockerCount)
	}
	if len(repo.items) != 5 {
		t.Fatalf("expected 5 items persisted, got %d", len(repo.items))
	}
	if len(publisher.extracted) != 1 || publisher.extracted[0].ItemCount != 5 {
		t.Fatalf("expected one action-item.extracted.v1 event with count 5, got %+v", publisher.extracted)
	}
	if len(publisher.failed) != 0 {
		t.Fatalf("expected no failure events on success, got %+v", publisher.failed)
	}
}

func TestExtractActionItemsUseCase_ExtractActionItems_SummaryFetchFailure(t *testing.T) {
	publisher := &fakePublisher{}
	uc := usecase.NewExtractActionItemsUseCase(
		&fakeSummaryClient{err: errors.New("ai-summary-service unreachable")},
		&fakeParticipantsClient{}, &fakeExtractor{}, &fakeRepository{}, publisher,
		logger.New("test", logger.LevelError),
	)

	if _, err := uc.ExtractActionItems(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when the summary can't be fetched")
	}
	// ExtractActionItems itself doesn't publish the failure event — see its
	// own doc comment: that's consumer.go's retry loop's job, on
	// exhaustion.
	if len(publisher.failed) != 0 {
		t.Fatalf("expected ExtractActionItems not to publish action-item.extraction-failed.v1 itself, got %+v", publisher.failed)
	}
}

func TestExtractActionItemsUseCase_ExtractActionItems_ExtractionFailure(t *testing.T) {
	summaryClient := &fakeSummaryClient{summary: &entity.Summary{MeetingID: "meeting-1", SummaryText: "hello"}}
	uc := usecase.NewExtractActionItemsUseCase(
		summaryClient, &fakeParticipantsClient{}, &fakeExtractor{err: errors.New("ollama unreachable")},
		&fakeRepository{}, &fakePublisher{}, logger.New("test", logger.LevelError),
	)

	if _, err := uc.ExtractActionItems(context.Background(), "org-1", "meeting-1"); err == nil {
		t.Fatal("expected an error when extraction fails")
	}
}

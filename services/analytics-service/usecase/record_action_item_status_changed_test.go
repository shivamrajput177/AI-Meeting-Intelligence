package usecase

import (
	"context"
	"testing"
	"time"
)

func TestRecordActionItemStatusChanged_Done(t *testing.T) {
	owner := "user-1"
	repo := &fakeRepository{}
	uc := NewRecordActionItemStatusChangedUseCase(repo)

	eventTime := time.Date(2026, 3, 18, 9, 0, 0, 0, time.UTC)
	if err := uc.RecordActionItemStatusChanged(context.Background(), "org-1", &owner, "done", eventTime); err != nil {
		t.Fatalf("RecordActionItemStatusChanged: %v", err)
	}
	if len(repo.closed) != 1 || repo.closed[0].ownerUserID != owner {
		t.Fatalf("expected 1 closed increment for owner-1, got %+v", repo.closed)
	}
}

func TestRecordActionItemStatusChanged_IgnoresNonDone(t *testing.T) {
	owner := "user-1"
	repo := &fakeRepository{}
	uc := NewRecordActionItemStatusChangedUseCase(repo)

	if err := uc.RecordActionItemStatusChanged(context.Background(), "org-1", &owner, "in_progress", time.Now()); err != nil {
		t.Fatalf("RecordActionItemStatusChanged: %v", err)
	}
	if len(repo.closed) != 0 {
		t.Errorf("expected no closed increment for in_progress, got %d", len(repo.closed))
	}
}

func TestRecordActionItemStatusChanged_IgnoresUnowned(t *testing.T) {
	repo := &fakeRepository{}
	uc := NewRecordActionItemStatusChangedUseCase(repo)

	if err := uc.RecordActionItemStatusChanged(context.Background(), "org-1", nil, "done", time.Now()); err != nil {
		t.Fatalf("RecordActionItemStatusChanged: %v", err)
	}
	if len(repo.closed) != 0 {
		t.Errorf("expected no closed increment for an unowned item, got %d", len(repo.closed))
	}
}

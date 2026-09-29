package usecase

import (
	"context"
	"testing"
)

func TestGetCompletionRate_ZeroOpened(t *testing.T) {
	repo := &fakeRepository{completionOpen: 0, completionDone: 0}
	uc := NewGetCompletionRateUseCase(repo)

	opened, closed, rate, err := uc.GetCompletionRate(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("GetCompletionRate: %v", err)
	}
	if opened != 0 || closed != 0 || rate != 0 {
		t.Errorf("GetCompletionRate = (%d, %d, %v), want (0, 0, 0)", opened, closed, rate)
	}
}

func TestGetCompletionRate_ComputesRatio(t *testing.T) {
	repo := &fakeRepository{completionOpen: 10, completionDone: 4}
	uc := NewGetCompletionRateUseCase(repo)

	opened, closed, rate, err := uc.GetCompletionRate(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("GetCompletionRate: %v", err)
	}
	if opened != 10 || closed != 4 || rate != 0.4 {
		t.Errorf("GetCompletionRate = (%d, %d, %v), want (10, 4, 0.4)", opened, closed, rate)
	}
}

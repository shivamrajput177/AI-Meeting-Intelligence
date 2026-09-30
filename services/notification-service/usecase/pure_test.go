package usecase

import (
	"strings"
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/notification-service/entity"
)

func TestRenderSummaryCompletedSlackText(t *testing.T) {
	got := renderSummaryCompletedSlackText("Q3 Planning")
	if !strings.Contains(got, "Q3 Planning") {
		t.Errorf("renderSummaryCompletedSlackText = %q, want it to mention the meeting title", got)
	}
}

func TestRenderSummaryReadyEmail(t *testing.T) {
	subject, body := renderSummaryReadyEmail("Q3 Planning")
	if !strings.Contains(subject, "Q3 Planning") {
		t.Errorf("subject = %q, want it to mention the meeting title", subject)
	}
	if !strings.Contains(body, "Q3 Planning") {
		t.Errorf("body = %q, want it to mention the meeting title", body)
	}
}

func TestRenderActionItemDigestSlackText(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{1, "1 action item extracted"},
		{3, "3 action items extracted"},
		{0, "0 action items extracted"},
	}
	for _, tt := range tests {
		got := renderActionItemDigestSlackText(tt.count, "Standup")
		if !strings.Contains(got, tt.want) {
			t.Errorf("renderActionItemDigestSlackText(%d, ...) = %q, want it to contain %q", tt.count, got, tt.want)
		}
	}
}

func TestShouldGiveUp(t *testing.T) {
	tests := []struct {
		attempts int
		want     bool
	}{
		{1, false},
		{MaxDispatchAttempts - 1, false},
		{MaxDispatchAttempts, true},
		{MaxDispatchAttempts + 1, true},
	}
	for _, tt := range tests {
		if got := shouldGiveUp(tt.attempts); got != tt.want {
			t.Errorf("shouldGiveUp(%d) = %v, want %v", tt.attempts, got, tt.want)
		}
	}
}

func TestMapMockStatusToActionItemStatus(t *testing.T) {
	tests := []struct {
		mockStatus string
		want       string
		wantOK     bool
	}{
		{entity.MockStatusToDo, actionItemStatusOpen, true},
		{entity.MockStatusInProgress, actionItemStatusInProgress, true},
		{entity.MockStatusDone, actionItemStatusDone, true},
		{"Blocked", "", false},
	}
	for _, tt := range tests {
		got, ok := mapMockStatusToActionItemStatus(tt.mockStatus)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("mapMockStatusToActionItemStatus(%q) = (%q, %v), want (%q, %v)", tt.mockStatus, got, ok, tt.want, tt.wantOK)
		}
	}
}

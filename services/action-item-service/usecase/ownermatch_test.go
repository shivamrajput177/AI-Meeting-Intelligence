package usecase

import (
	"testing"

	"github.com/shivamrajput177/ai-meeting-intelligence/services/action-item-service/entity"
)

func strPtr(s string) *string { return &s }

func TestMatchOwner(t *testing.T) {
	people := []entity.Participant{
		{UserID: strPtr("user-1"), Email: "shivam@example.com", DisplayName: "Shivam Rajput"},
		{UserID: strPtr("user-2"), Email: "priya@example.com", DisplayName: "Priya"},
		{UserID: nil, Email: "guest@example.com", DisplayName: "Guest Reviewer"}, // not a registered user
	}

	tests := []struct {
		name string
		raw  string
		want *string
	}{
		{"empty raw name matches nobody", "", nil},
		{"exact display name match, case-insensitive", "shivam rajput", strPtr("user-1")},
		{"display name with surrounding whitespace", "  Priya  ", strPtr("user-2")},
		{"falls back to email local part", "priya", strPtr("user-2")},
		{"no match for an unrelated name", "Someone Else", nil},
		{"participant with no user_id never matches even by name", "Guest Reviewer", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchOwner(tt.raw, people)
			if (got == nil) != (tt.want == nil) {
				t.Fatalf("matchOwner(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			if got != nil && *got != *tt.want {
				t.Fatalf("matchOwner(%q) = %q, want %q", tt.raw, *got, *tt.want)
			}
		})
	}
}

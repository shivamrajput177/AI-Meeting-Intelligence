package usecase

import (
	"testing"
	"time"
)

func TestDayOf(t *testing.T) {
	in := time.Date(2026, 3, 15, 23, 45, 0, 0, time.FixedZone("EST", -5*3600))
	got := dayOf(in)
	// 2026-03-15 23:45 EST == 2026-03-16 04:45 UTC, so the UTC day is the 16th.
	want := time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("dayOf(%v) = %v, want %v", in, got, want)
	}
}

func TestWeekOf(t *testing.T) {
	tests := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{"Monday itself", time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC), time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
		{"mid-week Wednesday", time.Date(2026, 3, 18, 10, 0, 0, 0, time.UTC), time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
		{"Sunday rolls back to same week's Monday", time.Date(2026, 3, 22, 23, 0, 0, 0, time.UTC), time.Date(2026, 3, 16, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := weekOf(tt.in); !got.Equal(tt.want) {
				t.Errorf("weekOf(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractTopics(t *testing.T) {
	got := extractTopics(
		[]string{"  Ship the API  ", ""},
		[]string{"Vendor Delay"},
		nil,
	)
	want := []string{"ship the api", "vendor delay"}
	if len(got) != len(want) {
		t.Fatalf("extractTopics = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("extractTopics[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExtractTopics_AllEmpty(t *testing.T) {
	if got := extractTopics(nil, nil, nil); got != nil {
		t.Errorf("extractTopics(nil, nil, nil) = %v, want nil", got)
	}
}

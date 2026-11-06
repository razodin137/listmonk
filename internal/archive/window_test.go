package archive

import (
	"strings"
	"testing"
	"time"
)

func TestWindowCutoff(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		years int
		want  time.Time
	}{
		{name: "no window", years: 0, want: time.Time{}},
		{name: "negative window", years: -1, want: time.Time{}},
		{name: "three years", years: 3, want: time.Date(2023, 11, 6, 12, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Window{Years: tt.years, Now: now}.Cutoff()
			if !got.Equal(tt.want) {
				t.Fatalf("Cutoff() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestWindowSkipReason(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	window := Window{Years: 3, Now: now}
	cutoff := window.Cutoff()

	tests := []struct {
		name  string
		date  time.Time
		valid bool
		want  string
	}{
		{name: "inside window", date: now, valid: true},
		{name: "on the cutoff", date: cutoff, valid: true},
		{
			name:  "before the cutoff",
			date:  cutoff.Add(-time.Second),
			valid: true,
			want:  "dated 2023-11-06, older than 3 years",
		},
		{name: "missing date", want: "no usable campaign date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := window.SkipReason(tt.date, tt.valid)
			if tt.want == "" {
				if got != "" {
					t.Fatalf("SkipReason() = %q, want no reason", got)
				}
				return
			}

			if !strings.Contains(got, tt.want) {
				t.Fatalf("SkipReason() = %q, want it to contain %q", got, tt.want)
			}
		})
	}
}

func TestWindowWithoutYearsKeepsAllDates(t *testing.T) {
	window := Window{Years: 0}
	if reason := window.SkipReason(time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC), true); reason != "" {
		t.Fatalf("SkipReason() = %q, want no reason without a window", reason)
	}
}

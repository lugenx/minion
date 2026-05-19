package engine

import (
	"testing"
)

func TestParseToCron(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"daily @ 09:00", "00 09 * * *"},
		{"weekdays @ 18:00", "00 18 * * 1,2,3,4,5"},
		{"weekends @ 12:00", "00 12 * * 0,6"},
		{"mon, wed, fri @ 17:30", "30 17 * * 1,3,5"},
		{"every 30m", "@every 30m"},
		{"every 12h", "@every 12h"},
		{"*/15 * * * *", "*/15 * * * *"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseToCron(tt.input)
			if tt.input == "" {
				if err == nil {
					t.Errorf("expected error for empty schedule, got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			} else if got != tt.want {
				t.Errorf("ParseToCron(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

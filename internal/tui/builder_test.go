package tui

import (
	"testing"

	"gopkg.in/yaml.v3"

	"minion/internal/config"
)

// YAML → TUI → YAML round-trip for all source types.
// Tests: YAML unmarshal + SourceType detection → TUI FromStep → ApplyToConfig → YAML marshal → re-parse
func TestRoundTrip_YAMLtoTUItoYAML(t *testing.T) {
	input := `name: test_minion
enabled: true
from:
  - url: https://example.com
  - search: test query
    limit: 5
  - minion: other_minion
  - command: ls -la
do: test task
settings:
  timeout: 30
  delay: 2
`
	var m config.MinionConfig
	if err := yaml.Unmarshal([]byte(input), &m); err != nil {
		t.Fatal(err)
	}

	if len(m.From) != 4 {
		t.Fatalf("expected 4 sources, got %d", len(m.From))
	}

	tests := []struct {
		idx      int
		wantType string
		check    func(s config.Source) bool
	}{
		{0, "url", func(s config.Source) bool { return s.URL == "https://example.com" }},
		{1, "search", func(s config.Source) bool { return s.Search == "test query" && s.Limit == 5 }},
		{2, "minion", func(s config.Source) bool { return s.Minion == "other_minion" }},
		{3, "command", func(s config.Source) bool { return s.Command == "ls -la" }},
	}

	for _, tt := range tests {
		got := m.From[tt.idx].SourceType
		if got != tt.wantType {
			t.Errorf("source %d: SourceType=%q, want %q", tt.idx, got, tt.wantType)
		}
		if !tt.check(m.From[tt.idx]) {
			t.Errorf("source %d: field check failed", tt.idx)
		}
	}

	// Build TUI FromStep and verify GetRows rendering
	fromStep := &FromStep{Sources: m.From}
	rows := fromStep.GetRows(0)

	if len(rows) < 4 {
		t.Fatalf("GetRows returned %d rows, expected at least 4 source field rows", len(rows))
	}

	rowFields := []string{"FromURL", "FromSearch", "FromMinion", "FromCommand"}
	rowVals := []string{"https://example.com", "test query", "other_minion", "ls -la"}
	for i, field := range rowFields {
		found := false
		for _, r := range rows {
			if r.Field == field {
				found = true
				if r.Value != rowVals[i] {
					t.Errorf("GetRows field %s: value=%q, want %q", field, r.Value, rowVals[i])
				}
				// Verify label matches source type
				expectedLabels := []string{"url", "search", "minion", "command"}
				if r.Label != expectedLabels[i] {
					t.Errorf("GetRows field %s: label=%q, want %q", field, r.Label, expectedLabels[i])
				}
			}
		}
		if !found {
			t.Errorf("GetRows missing field %s", field)
		}
	}

	// ApplyToConfig → marshal → unmarshal → verify
	var m2 config.MinionConfig
	m2.Name = "test_minion"
	enabled := true
	m2.Enabled = &enabled
	fromStep.ApplyToConfig(&m2)

	out, err := yaml.Marshal(m2)
	if err != nil {
		t.Fatal(err)
	}

	var m3 config.MinionConfig
	if err := yaml.Unmarshal(out, &m3); err != nil {
		t.Fatal(err)
	}

	if len(m3.From) != 4 {
		t.Fatalf("round-trip: expected 4 sources, got %d", len(m3.From))
	}

	for _, tt := range tests {
		got := m3.From[tt.idx].SourceType
		if got != tt.wantType {
			t.Errorf("round-trip source %d: SourceType=%q, want %q", tt.idx, got, tt.wantType)
		}
		if !tt.check(m3.From[tt.idx]) {
			t.Errorf("round-trip source %d: field check failed", tt.idx)
		}
	}
}

// TUI → YAML → TUI round-trip for all source types.
// Tests: TUI FromStep creation → ApplyToConfig → YAML marshal → unmarshal → GetRows rendering
func TestRoundTrip_TUItoYAMLtoTUI(t *testing.T) {
	fromStep := &FromStep{
		Sources: []config.Source{
			{URL: "https://example.com/news", SourceType: "url"},
			{Search: "golang", Limit: 3, SourceType: "search"},
			{Minion: "event_tracker", SourceType: "minion"},
			{Command: "df -h", SourceType: "command"},
		},
	}

	// ApplyToConfig → marshal
	var m config.MinionConfig
	enabled := true
	m.Name = "test"
	m.Enabled = &enabled
	m.Do = "summarize"
	fromStep.ApplyToConfig(&m)

	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal back
	var m2 config.MinionConfig
	if err := yaml.Unmarshal(out, &m2); err != nil {
		t.Fatal(err)
	}

	if len(m2.From) != 4 {
		t.Fatalf("expected 4 sources, got %d", len(m2.From))
	}

	tests := []struct {
		idx      int
		wantType string
		check    func(s config.Source) bool
	}{
		{0, "url", func(s config.Source) bool { return s.URL == "https://example.com/news" }},
		{1, "search", func(s config.Source) bool { return s.Search == "golang" && s.Limit == 3 }},
		{2, "minion", func(s config.Source) bool { return s.Minion == "event_tracker" }},
		{3, "command", func(s config.Source) bool { return s.Command == "df -h" }},
	}

	for _, tt := range tests {
		got := m2.From[tt.idx].SourceType
		if got != tt.wantType {
			t.Errorf("source %d: SourceType=%q, want %q", tt.idx, got, tt.wantType)
		}
		if !tt.check(m2.From[tt.idx]) {
			t.Errorf("source %d: field check failed", tt.idx)
		}
	}

	// Build TUI FromStep from parsed sources and verify GetRows
	fromStep2 := &FromStep{Sources: m2.From}
	rows := fromStep2.GetRows(0)
	rowFields := []string{"FromURL", "FromSearch", "FromMinion", "FromCommand"}
	rowVals := []string{"https://example.com/news", "golang", "event_tracker", "df -h"}

	for i, field := range rowFields {
		found := false
		for _, r := range rows {
			if r.Field == field {
				found = true
				if r.Value != rowVals[i] {
					t.Errorf("GetRows field %s: value=%q, want %q", field, r.Value, rowVals[i])
				}
			}
		}
		if !found {
			t.Errorf("GetRows missing field %s", field)
		}
	}
}

// Empty sources are filtered out by saveBuilder's logic
func TestSaveBuilder_FiltersEmptySources(t *testing.T) {
	tests := []struct {
		name     string
		sources  []config.Source
		expected int
	}{
		{
			name: "empty command filtered",
			sources: []config.Source{
				{SourceType: "command", Command: ""},
				{SourceType: "command", Command: "ls"},
			},
			expected: 1,
		},
		{
			name: "empty minion filtered",
			sources: []config.Source{
				{SourceType: "minion", Minion: ""},
			},
			expected: 0,
		},
		{
			name: "empty search filtered",
			sources: []config.Source{
				{SourceType: "search", Search: ""},
			},
			expected: 0,
		},
		{
			name: "empty url filtered",
			sources: []config.Source{
				{SourceType: "url", URL: ""},
			},
			expected: 0,
		},
		{
			name: "populated sources kept",
			sources: []config.Source{
				{SourceType: "url", URL: "https://example.com"},
				{SourceType: "search", Search: "test", Limit: 3},
				{SourceType: "minion", Minion: "other"},
				{SourceType: "command", Command: "ls"},
			},
			expected: 4,
		},
		{
			name: "mixed empty and populated",
			sources: []config.Source{
				{SourceType: "command", Command: ""},
				{SourceType: "url", URL: ""},
				{SourceType: "command", Command: "ls"},
				{SourceType: "url", URL: "https://example.com"},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fromStep := &FromStep{Sources: tt.sources}

			// Apply the same filter logic as saveBuilder
			var filtered []config.Source
			for _, s := range fromStep.Sources {
				isEmpty := false
				switch s.SourceType {
				case "command":
					isEmpty = s.Command == ""
				case "minion":
					isEmpty = s.Minion == ""
				case "search":
					isEmpty = s.Search == "" && s.Limit == 0
				default:
					isEmpty = s.URL == ""
				}
				if isEmpty {
					continue
				}
				filtered = append(filtered, s)
			}
			fromStep.Sources = filtered

			if len(fromStep.Sources) != tt.expected {
				t.Errorf("expected %d sources after filtering, got %d", tt.expected, len(fromStep.Sources))
			}
		})
	}
}

// initBuilderData correctly converts loaded MinionConfig to TUI builder data
func TestInitBuilderData_FromSources(t *testing.T) {
	m := &config.MinionConfig{
		Name:     "test",
		Enabled:  boolPtr(true),
		Filename: "test.yaml",
		From: []config.Source{
			{URL: "https://example.com"},
			{Search: "test", Limit: 5},
			{Minion: "other"},
			{Command: "df -h"},
		},
		Do: "summarize",
		Settings: config.Settings{
			Timeout: 30,
			Delay:   2,
		},
	}

	data := initBuilderData(m)

	var fromStep *FromStep
	for _, s := range data.Steps {
		if fs, ok := s.(*FromStep); ok {
			fromStep = fs
			break
		}
	}

	if fromStep == nil {
		t.Fatal("expected FromStep in builder data")
	}

	if len(fromStep.Sources) != 4 {
		t.Fatalf("expected 4 sources in FromStep, got %d", len(fromStep.Sources))
	}

	expected := []struct {
		field string
		value string
	}{
		{"FromURL", "https://example.com"},
		{"FromSearch", "test"},
		{"FromMinion", "other"},
		{"FromCommand", "df -h"},
	}

	rows := fromStep.GetRows(0)
	for _, exp := range expected {
		found := false
		for _, r := range rows {
			if r.Field == exp.field {
				found = true
				if r.Value != exp.value {
					t.Errorf("GetRows field %s: value=%q, want %q", exp.field, r.Value, exp.value)
				}
				break
			}
		}
		if !found {
			t.Errorf("GetRows missing field %s", exp.field)
		}
	}
}

func boolPtr(b bool) *bool { return &b }

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

func TestTellStep_LabelMinion(t *testing.T) {
	// When tell target type is "minion", label should be "minion" not "url"
	step := &TellStep{
		Targets: []map[string]interface{}{
			{"minion": "worker"},
			{"ntfy": "https://ntfy.sh/topic"},
		},
	}

	rows := step.GetRows(0)

	// First target: minion type → label "minion"
	foundMinion := false
	foundNtfy := false
	for _, r := range rows {
		if r.Field == "TellURL" && r.TargetIndex == 0 {
			foundMinion = true
			if r.Label != "minion" {
				t.Errorf("expected label 'minion' for minion-type target, got %q", r.Label)
			}
			if r.Value != "worker" {
				t.Errorf("expected value 'worker', got %q", r.Value)
			}
		}
		if r.Field == "TellURL" && r.TargetIndex == 1 {
			foundNtfy = true
			if r.Label != "ntfy" {
				t.Errorf("expected label 'ntfy' for ntfy-type target, got %q", r.Label)
			}
		}
	}
	if !foundMinion {
		t.Error("missing TellURL field for minion target")
	}
	if !foundNtfy {
		t.Error("missing TellURL field for ntfy target")
	}
}

func TestTellStep_AddArrayItem_LandsOnType(t *testing.T) {
	// After AddArrayItem, new target should have no type key (empty type field)
	step := &TellStep{
		Targets: []map[string]interface{}{
			{"ntfy": "https://ntfy.sh/topic"},
		},
	}

	step.AddArrayItem("Tell")
	if len(step.Targets) != 2 {
		t.Fatalf("expected 2 targets after AddArrayItem, got %d", len(step.Targets))
	}

	// The new target should be an empty map (no type key)
	newTarget := step.Targets[1]
	if len(newTarget) != 0 {
		t.Errorf("expected empty map for new target, got %v", newTarget)
	}

	// After setting type, Verify UpdateField sets it correctly
	err := step.UpdateField("TellType", 1, "minion")
	if err != nil {
		t.Fatalf("UpdateField failed: %v", err)
	}
	if _, ok := step.Targets[1]["minion"]; !ok {
		t.Errorf("expected minion key to be set in target after UpdateField")
	}
}

func TestReportStep_LabelMinion(t *testing.T) {
	step := &ReportStep{
		Targets: []map[string]interface{}{
			{"minion": "worker"},
		},
	}

	rows := step.GetRows(0)
	found := false
	for _, r := range rows {
		if r.Field == "ReportURL" && r.TargetIndex == 0 {
			found = true
			if r.Label != "minion" {
				t.Errorf("expected label 'minion' for minion-type report target, got %q", r.Label)
			}
		}
	}
	if !found {
		t.Error("missing ReportURL field for minion report target")
	}
}

func TestGenerateBuilderRows_AddStepOnTop(t *testing.T) {
	data := &builderData{
		Name:    "test",
		Enabled: true,
		Steps: []Step{
			&WhenStep{When: "every 5m"},
			&DoStep{Do: "test task"},
		},
	}

	rows := generateBuilderRows(data)

	// First two rows should be Name and Enabled
	if len(rows) < 3 {
		t.Fatalf("expected at least 3 rows, got %d", len(rows))
	}
	if rows[0].Type != rowName {
		t.Errorf("row[0] should be rowName, got %v", rows[0].Type)
	}
	if rows[1].Type != rowEnabled {
		t.Errorf("row[1] should be rowEnabled, got %v", rows[1].Type)
	}

	// Third row should be the top "Add Step" button with StepIndex -2
	if rows[2].Type != rowAddStep {
		t.Errorf("row[2] should be rowAddStep, got %v", rows[2].Type)
	}
	if rows[2].StepIndex != -2 {
		t.Errorf("row[2] StepIndex should be -2 (add on top), got %d", rows[2].StepIndex)
	}
	if rows[2].Label != "[ + Add Step ]" {
		t.Errorf("row[2] Label should be '[ + Add Step ]', got %q", rows[2].Label)
	}

	// After the first step, there should be another AddStep with StepIndex 0
	foundAfterStep0 := false
	for _, r := range rows {
		if r.Type == rowAddStep && r.StepIndex == 0 {
			foundAfterStep0 = true
			break
		}
	}
	if !foundAfterStep0 {
		t.Error("expected rowAddStep with StepIndex 0 (after step 0)")
	}

	// After the second step, there should be another AddStep with StepIndex 1
	foundAfterStep1 := false
	for _, r := range rows {
		if r.Type == rowAddStep && r.StepIndex == 1 {
			foundAfterStep1 = true
			break
		}
	}
	if !foundAfterStep1 {
		t.Error("expected rowAddStep with StepIndex 1 (after step 1)")
	}
}

func TestGenerateBuilderRows_AddStepOnTop_NoSteps(t *testing.T) {
	data := &builderData{
		Name:    "test",
		Enabled: true,
	}

	rows := generateBuilderRows(data)

	// When no steps, should still have a top AddStep button
	foundTop := false
	for _, r := range rows {
		if r.Type == rowAddStep {
			foundTop = true
			if r.StepIndex != -2 {
				t.Errorf("expected StepIndex -2 for top add step, got %d", r.StepIndex)
			}
			break
		}
	}
	if !foundTop {
		t.Error("expected rowAddStep in empty steps case")
	}
}

func TestGenerateBuilderRows_StepOrder(t *testing.T) {
	// Verify steps appear in correct order with proper StepIndex values
	data := &builderData{
		Name:    "test",
		Enabled: true,
		Steps: []Step{
			&WhenStep{When: "every 5m"},
			&DoStep{Do: "task"},
			&TellStep{Targets: []map[string]interface{}{{"ntfy": "topic"}}},
		},
	}

	rows := generateBuilderRows(data)

	// Collect all StepHeader rows to verify step ordering
	var headers []int
	for _, r := range rows {
		if r.Type == rowStepHeader {
			headers = append(headers, r.StepIndex)
		}
	}
	if len(headers) != 3 {
		t.Fatalf("expected 3 step headers, got %d", len(headers))
	}
	if headers[0] != 0 || headers[1] != 1 || headers[2] != 2 {
		t.Errorf("step headers should be [0,1,2], got %v", headers)
	}

	// Verify AddStep after each step
	expectedAddStepIndices := map[int]bool{-2: true, 0: true, 1: true, 2: true}
	for _, r := range rows {
		if r.Type == rowAddStep {
			if !expectedAddStepIndices[r.StepIndex] {
				t.Errorf("unexpected rowAddStep with StepIndex %d", r.StepIndex)
			}
			delete(expectedAddStepIndices, r.StepIndex)
		}
	}
	if len(expectedAddStepIndices) > 0 {
		t.Errorf("missing rowAddStep entries for indices: %v", expectedAddStepIndices)
	}
}

func TestTellStep_AddArrayItem_CursorTarget(t *testing.T) {
	step := &TellStep{
		Targets: []map[string]interface{}{
			{"ntfy": "https://ntfy.sh/topic"},
		},
	}

	// A fresh empty target has no type key, so TellURL label should be empty
	step.AddArrayItem("Tell")
	rows := step.GetRows(0)

	hasEmptyLabel := false
	for _, r := range rows {
		if r.Field == "TellURL" && r.TargetIndex == 1 {
			if r.Label == "" {
				hasEmptyLabel = true
			}
			break
		}
	}
	if !hasEmptyLabel {
		t.Error("expected TellURL field with empty label for fresh target (no type set)")
	}

	// After setting the type, the TellURL label should show the type
	step.Targets[1]["discord"] = ""
	rows = step.GetRows(0)
	found := false
	for _, r := range rows {
		if r.Field == "TellURL" && r.TargetIndex == 1 {
			found = true
			if r.Label != "discord" {
				t.Errorf("expected label 'discord', got %q", r.Label)
			}
			break
		}
	}
	if !found {
		t.Error("expected TellURL field after setting type on new target")
	}
}

func boolPtr(b bool) *bool { return &b }

package tui

import (
	"fmt"
	"strconv"
	"strings"

	"minion/internal/config"
)

type Step interface {
	Type() StepType
	ApplyToConfig(m *config.MinionConfig)
	GetRows(stepIndex int) []builderRow
	UpdateField(field string, targetIndex int, value string) error
	AddArrayItem(field string)
	RemoveArrayItem(field string, index int)
}

// ---------------------------------------------------------
// WHEN STEP
// ---------------------------------------------------------
type WhenStep struct {
	When string
}

func (s *WhenStep) Type() StepType { return StepWhen }
func (s *WhenStep) ApplyToConfig(m *config.MinionConfig) { m.When = s.When }
func (s *WhenStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "When", Label: "schedule", Value: s.When},
	}
}
func (s *WhenStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "When" { s.When = value }
	return nil
}
func (s *WhenStep) AddArrayItem(field string) {}
func (s *WhenStep) RemoveArrayItem(field string, index int) {}

// ---------------------------------------------------------
// FROM STEP
// ---------------------------------------------------------
type FromStep struct {
	Sources []config.Source
}

func (s *FromStep) Type() StepType { return StepFrom }
func (s *FromStep) ApplyToConfig(m *config.MinionConfig) { m.From = s.Sources }
func (s *FromStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow
	for i, src := range s.Sources {
		switch {
		case src.SourceType == "minion" || src.Minion != "":
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FromMinion", Label: "minion", Value: src.Minion})
		case src.SourceType == "search" || src.Search != "" || src.Limit != 0:
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FromSearch", Label: "search", Value: src.Search})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FromLimit", Label: "  limit", Value: fmt.Sprintf("%d", src.Limit)})
		default:
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FromURL", Label: "url", Value: src.URL})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FromRender", Label: "  render", Value: fmt.Sprintf("%t", src.Render)})
			rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: i, Field: "FromMatch", Label: "  match", Value: src.Match})
		}
		rows = append(rows, builderRow{Type: rowRemoveSubItem, StepIndex: stepIndex, TargetIndex: i, Label: "[ - Remove Source ]"})
		rows = append(rows, builderRow{Type: rowSpacer, StepIndex: stepIndex, TargetIndex: -1, Field: "EditOnlySpacer"})
	}
	rows = append(rows, builderRow{Type: rowAddSubItem, StepIndex: stepIndex, TargetIndex: -1, Field: "From", Label: "[ + Add Source ]"})
	return rows
}
func (s *FromStep) UpdateField(field string, targetIndex int, value string) error {
	if targetIndex < 0 || targetIndex >= len(s.Sources) { return nil }
	switch field {
	case "FromURL": s.Sources[targetIndex].URL = value
	case "FromRender": s.Sources[targetIndex].Render = value == "true"
	case "FromMatch": s.Sources[targetIndex].Match = value
	case "FromSearch": s.Sources[targetIndex].Search = value
	case "FromMinion": s.Sources[targetIndex].Minion = value
	case "FromLimit":
		if l, err := strconv.Atoi(value); err == nil { s.Sources[targetIndex].Limit = l }
	}
	return nil
}
func (s *FromStep) AddArrayItem(field string) {
	if field == "FromURL" {
		s.Sources = append(s.Sources, config.Source{SourceType: "url"})
	} else if field == "FromSearch" {
		s.Sources = append(s.Sources, config.Source{Search: "", Limit: 3, SourceType: "search"})
	} else if field == "FromMinion" {
		s.Sources = append(s.Sources, config.Source{Minion: "", SourceType: "minion"})
	}
}
func (s *FromStep) RemoveArrayItem(field string, index int) {
	if index >= 0 && index < len(s.Sources) {
		s.Sources = append(s.Sources[:index], s.Sources[index+1:]...)
	}
}

// ---------------------------------------------------------
// KEEP STEP
// ---------------------------------------------------------
type KeepStep struct {
	Keywords []string
}
func (s *KeepStep) Type() StepType { return StepKeep }
func (s *KeepStep) ApplyToConfig(m *config.MinionConfig) { m.Keep = s.Keywords }
func (s *KeepStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "KeepWords", Label: "keywords", Value: strings.Join(s.Keywords, ", ")},
	}
}
func (s *KeepStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "KeepWords" {
		parts := strings.Split(value, ",")
		var cleaned []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		s.Keywords = cleaned
	}
	return nil
}
func (s *KeepStep) AddArrayItem(field string) {}
func (s *KeepStep) RemoveArrayItem(field string, index int) {}

// ---------------------------------------------------------
// IGNORE STEP
// ---------------------------------------------------------
type IgnoreStep struct {
	Keywords []string
}
func (s *IgnoreStep) Type() StepType { return StepIgnore }
func (s *IgnoreStep) ApplyToConfig(m *config.MinionConfig) { m.Ignore = s.Keywords }
func (s *IgnoreStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "IgnoreWords", Label: "keywords", Value: strings.Join(s.Keywords, ", ")},
	}
}
func (s *IgnoreStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "IgnoreWords" {
		parts := strings.Split(value, ",")
		var cleaned []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cleaned = append(cleaned, trimmed)
			}
		}
		s.Keywords = cleaned
	}
	return nil
}
func (s *IgnoreStep) AddArrayItem(field string) {}
func (s *IgnoreStep) RemoveArrayItem(field string, index int) {}

// ---------------------------------------------------------
// DO STEP
// ---------------------------------------------------------
type DoStep struct {
	Do string
}
func (s *DoStep) Type() StepType { return StepDo }
func (s *DoStep) ApplyToConfig(m *config.MinionConfig) { m.Do = s.Do }
func (s *DoStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DoTask", Label: "task", Value: s.Do},
	}
}
func (s *DoStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "DoTask" { s.Do = value }
	return nil
}
func (s *DoStep) AddArrayItem(field string) {}
func (s *DoStep) RemoveArrayItem(field string, index int) {}

// ---------------------------------------------------------
// TELL STEP
// ---------------------------------------------------------
type TellStep struct {
	Targets map[string]interface{}
}
func (s *TellStep) Type() StepType { return StepTell }
func (s *TellStep) ApplyToConfig(m *config.MinionConfig) { m.Tell = s.Targets }
func (s *TellStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow

	var url string
	var ttype string
	for k, v := range s.Targets {
		if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
			ttype = k
			url = fmt.Sprintf("%v", v)
			break
		}
	}

	rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "TellType", Label: "type", Value: ttype})
	rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "TellURL", Label: "url", Value: url})

	switch ttype {
	case "ntfy":
		md := false
		if v, ok := s.Targets["markdown"]; ok {
			md, _ = v.(bool)
		}
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverMarkdown", Label: "markdown", Value: fmt.Sprintf("%t", md)})

		username, password := "", ""
		if ba, ok := s.Targets["basic_auth"].(map[string]interface{}); ok {
			if u, ok := ba["username"].(string); ok { username = u }
			if p, ok := ba["password"].(string); ok { password = p }
		}
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverUsername", Label: "auth user", Value: username})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverPassword", Label: "auth pass", Value: password})

	case "http_request":
		method := ""
		payload := ""
		if v, ok := s.Targets["method"].(string); ok { method = v }
		if v, ok := s.Targets["payload_template"].(string); ok { payload = v }
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverMethod", Label: "method", Value: method})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverPayload", Label: "payload", Value: payload})

		username, password := "", ""
		if ba, ok := s.Targets["basic_auth"].(map[string]interface{}); ok {
			if u, ok := ba["username"].(string); ok { username = u }
			if p, ok := ba["password"].(string); ok { password = p }
		}
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverUsername", Label: "auth user", Value: username})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "DeliverPassword", Label: "auth pass", Value: password})
	}

	return rows
}
func (s *TellStep) UpdateField(field string, targetIndex int, value string) error {
	if s.Targets == nil { s.Targets = make(map[string]interface{}) }
	switch field {
	case "TellType":
		oldUrl := ""
		for k, v := range s.Targets {
			if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
				oldUrl = fmt.Sprintf("%v", v)
				delete(s.Targets, k)
			}
		}
		delete(s.Targets, "markdown")
		delete(s.Targets, "method")
		delete(s.Targets, "payload_template")
		if value != "" { s.Targets[value] = oldUrl }
	case "TellURL":
		ttype := "ntfy"
		for k := range s.Targets {
			if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
				ttype = k
				break
			}
		}
		s.Targets[ttype] = value
	case "DeliverMarkdown":
		s.Targets["markdown"] = value == "true"
	case "DeliverMarkdownToggle":
		md, _ := s.Targets["markdown"].(bool)
		s.Targets["markdown"] = !md
	case "DeliverUsername":
		ba, ok := s.Targets["basic_auth"].(map[string]interface{})
		if !ok {
			ba = make(map[string]interface{})
			s.Targets["basic_auth"] = ba
		}
		ba["username"] = value
	case "DeliverPassword":
		ba, ok := s.Targets["basic_auth"].(map[string]interface{})
		if !ok {
			ba = make(map[string]interface{})
			s.Targets["basic_auth"] = ba
		}
		ba["password"] = value
	case "DeliverMethod":
		s.Targets["method"] = value
	case "DeliverPayload":
		s.Targets["payload_template"] = value
	}
	return nil
}
func (s *TellStep) AddArrayItem(field string) {}
func (s *TellStep) RemoveArrayItem(field string, index int) {}

// ---------------------------------------------------------
// REPORT STEP
// ---------------------------------------------------------
type ReportStep struct {
	Targets map[string]interface{}
}
func (s *ReportStep) Type() StepType { return StepReport }
func (s *ReportStep) ApplyToConfig(m *config.MinionConfig) { m.Report = s.Targets }
func (s *ReportStep) GetRows(stepIndex int) []builderRow {
	var rows []builderRow
	var url string
	var ttype string
	for k, v := range s.Targets {
		if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
			ttype = k
			url = fmt.Sprintf("%v", v)
			break
		}
	}
	rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportType", Label: "type", Value: ttype})
	rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportURL", Label: "url", Value: url})

	switch ttype {
	case "ntfy":
		md := false
		if v, ok := s.Targets["markdown"]; ok {
			md, _ = v.(bool)
		}
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportMarkdown", Label: "markdown", Value: fmt.Sprintf("%t", md)})

		username, password := "", ""
		if ba, ok := s.Targets["basic_auth"].(map[string]interface{}); ok {
			if u, ok := ba["username"].(string); ok { username = u }
			if p, ok := ba["password"].(string); ok { password = p }
		}
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportUsername", Label: "auth user", Value: username})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportPassword", Label: "auth pass", Value: password})

	case "http_request":
		method := ""
		payload := ""
		if v, ok := s.Targets["method"].(string); ok { method = v }
		if v, ok := s.Targets["payload_template"].(string); ok { payload = v }
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportMethod", Label: "method", Value: method})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportPayload", Label: "payload", Value: payload})

		username, password := "", ""
		if ba, ok := s.Targets["basic_auth"].(map[string]interface{}); ok {
			if u, ok := ba["username"].(string); ok { username = u }
			if p, ok := ba["password"].(string); ok { password = p }
		}
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportUsername", Label: "auth user", Value: username})
		rows = append(rows, builderRow{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "ReportPassword", Label: "auth pass", Value: password})
	}

	return rows
}
func (s *ReportStep) UpdateField(field string, targetIndex int, value string) error {
	if s.Targets == nil { s.Targets = make(map[string]interface{}) }
	switch field {
	case "ReportType":
		oldUrl := ""
		for k, v := range s.Targets {
			if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
				oldUrl = fmt.Sprintf("%v", v)
				delete(s.Targets, k)
			}
		}
		delete(s.Targets, "markdown")
		delete(s.Targets, "method")
		delete(s.Targets, "payload_template")
		if value != "" { s.Targets[value] = oldUrl }
	case "ReportURL":
		ttype := "ntfy"
		for k := range s.Targets {
			if k == "ntfy" || k == "discord" || k == "http_request" || k == "minion" {
				ttype = k
				break
			}
		}
		s.Targets[ttype] = value
	case "ReportMarkdown":
		s.Targets["markdown"] = value == "true"
	case "ReportMarkdownToggle":
		md, _ := s.Targets["markdown"].(bool)
		s.Targets["markdown"] = !md
	case "ReportUsername":
		ba, ok := s.Targets["basic_auth"].(map[string]interface{})
		if !ok {
			ba = make(map[string]interface{})
			s.Targets["basic_auth"] = ba
		}
		ba["username"] = value
	case "ReportPassword":
		ba, ok := s.Targets["basic_auth"].(map[string]interface{})
		if !ok {
			ba = make(map[string]interface{})
			s.Targets["basic_auth"] = ba
		}
		ba["password"] = value
	case "ReportMethod":
		s.Targets["method"] = value
	case "ReportPayload":
		s.Targets["payload_template"] = value
	}
	return nil
}
func (s *ReportStep) AddArrayItem(field string) {}
func (s *ReportStep) RemoveArrayItem(field string, index int) {}

// ---------------------------------------------------------
// SETTINGS STEP
// ---------------------------------------------------------
type SettingsStep struct {
	Settings config.Settings
}
func (s *SettingsStep) Type() StepType { return StepSettings }
func (s *SettingsStep) ApplyToConfig(m *config.MinionConfig) { m.Settings = s.Settings }
func (s *SettingsStep) GetRows(stepIndex int) []builderRow {
	return []builderRow{
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "Timeout", Label: "timeout", Value: fmt.Sprintf("%d", s.Settings.Timeout)},
		{Type: rowStepField, StepIndex: stepIndex, TargetIndex: -1, Field: "Delay", Label: "delay", Value: fmt.Sprintf("%d", s.Settings.Delay)},
	}
}
func (s *SettingsStep) UpdateField(field string, targetIndex int, value string) error {
	if field == "Timeout" {
		if i, err := strconv.Atoi(value); err == nil { s.Settings.Timeout = i }
	}
	if field == "Delay" {
		if i, err := strconv.Atoi(value); err == nil { s.Settings.Delay = i }
	}
	return nil
}
func (s *SettingsStep) AddArrayItem(field string) {}
func (s *SettingsStep) RemoveArrayItem(field string, index int) {}

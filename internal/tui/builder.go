package tui

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"minion/internal/config"
)

type StepType string

const (
	StepSchedule StepType = "schedule"
	StepSearch   StepType = "search"
	StepBrowse   StepType = "browse"
	StepFilter   StepType = "filter"
	StepScrape   StepType = "scrape"
	StepStudy    StepType = "study"
	StepDeliver  StepType = "deliver"
	StepReceive  StepType = "receive"
	StepReport   StepType = "report"
)

type BrowseTarget struct {
	URL    string
	Match  string
	Render bool
}

type builderData struct {
	Name     string
	Enabled  bool
	Filename string
	IsNew    bool
	Steps    []Step
}

func loadRawMinionForEditing(filename string) *config.MinionConfig {
	path := filepath.Join(config.MinionsDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var m config.MinionConfig
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil
	}
	
	if m.Enabled == nil {
		defaultEnabled := true
		m.Enabled = &defaultEnabled
	}
	
	m.Filename = filename
	return &m
}

func initBuilderData(m *config.MinionConfig) *builderData {
	data := &builderData{
		Enabled: true,
		IsNew:   m == nil,
	}

	if m != nil {
		data.Name = m.Name
		if m.Enabled != nil {
			data.Enabled = *m.Enabled
		}
		data.Filename = m.Filename

		for _, stepMap := range m.Mission {
			var step Step
			if _, ok := stepMap["schedule"]; ok {
				step = &ScheduleStep{}
			} else if _, ok := stepMap["search"]; ok {
				step = &SearchStep{}
			} else if _, ok := stepMap["browse"]; ok {
				step = &BrowseStep{}
			} else if _, ok := stepMap["filter"]; ok {
				step = &FilterStep{}
			} else if _, ok := stepMap["scrape"]; ok {
				step = &ScrapeStep{}
			} else if _, ok := stepMap["study"]; ok {
				step = &StudyStep{}
			} else if _, ok := stepMap["deliver"]; ok {
				step = &DeliverStep{}
			} else if _, ok := stepMap["receive"]; ok {
				step = &ReceiveStep{}
			} else if _, ok := stepMap["report"]; ok {
				step = &ReportStep{}
			}

			if step != nil {
				step.FromYAML(stepMap)
				data.Steps = append(data.Steps, step)
			}
		}
	}
	return data
}

func saveBuilder(data *builderData) error {
	filename := data.Filename
	if data.IsNew || filename == "" {
		safeName := strings.ToLower(strings.ReplaceAll(data.Name, " ", "_"))
		if safeName == "" {
			safeName = "new_minion"
		}
		filename = safeName + ".yaml"
	}

	var mission []map[string]interface{}

	for _, step := range data.Steps {
		yamlData := step.ToYAML()
		if yamlData != nil {
			mission = append(mission, yamlData)
		}
	}

	mConfig := config.MinionConfig{
		Name:    data.Name,
		Enabled: &data.Enabled,
		Mission: mission,
	}

	yamlData, err := yaml.Marshal(mConfig)
	if err != nil {
		return err
	}

	path := filepath.Join(config.MinionsDir, filename)
	return os.WriteFile(path, yamlData, 0644)
}

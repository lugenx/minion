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
	StepWhen     StepType = "when"
	StepFrom     StepType = "from"
	StepKeep     StepType = "keep"
	StepIgnore   StepType = "ignore"
	StepDo       StepType = "do"
	StepTell     StepType = "tell"
	StepReport   StepType = "report"
	StepSettings StepType = "settings"
)

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

		if m.When != "" {
			data.Steps = append(data.Steps, &WhenStep{When: m.When})
		}

		if len(m.From) > 0 {
			data.Steps = append(data.Steps, &FromStep{Sources: m.From})
		}

		if len(m.Keep) > 0 {
			data.Steps = append(data.Steps, &KeepStep{Keywords: m.Keep})
		}

		if len(m.Ignore) > 0 {
			data.Steps = append(data.Steps, &IgnoreStep{Keywords: m.Ignore})
		}

		if m.Do != "" {
			data.Steps = append(data.Steps, &DoStep{Do: m.Do})
		}

		if len(m.Tell) > 0 {
			data.Steps = append(data.Steps, &TellStep{Targets: m.Tell})
		}

		if len(m.Report) > 0 {
			data.Steps = append(data.Steps, &ReportStep{Targets: m.Report})
		}

		if m.Settings.Timeout > 0 || m.Settings.Delay > 0 || m.Settings.Model != "" {
			data.Steps = append(data.Steps, &SettingsStep{Settings: m.Settings})
		}
	} else {
		data.Steps = append(data.Steps,
			&WhenStep{},
			&FromStep{},
			&KeepStep{},
			&IgnoreStep{},
			&DoStep{},
			&TellStep{},
			&ReportStep{},
			&SettingsStep{Settings: config.Settings{Timeout: 15, Delay: 2}},
		)
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

	mConfig := config.MinionConfig{
		Name:    data.Name,
		Enabled: &data.Enabled,
	}

	for _, step := range data.Steps {
		step.ApplyToConfig(&mConfig)
	}

	yamlData, err := yaml.Marshal(mConfig)
	if err != nil {
		return err
	}

	path := filepath.Join(config.MinionsDir, filename)
	return os.WriteFile(path, yamlData, 0644)
}

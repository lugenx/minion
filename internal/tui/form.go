package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v3"

	"minion/internal/config"
)

type formData struct {
	Name       string
	Enabled    bool
	Schedule   string
	Search     string
	URL        string
	Task       string
	WebhookURL string
	Filename   string
	IsNew      bool
}

func buildForm(m *config.MinionConfig) (*huh.Form, *formData) {
	data := &formData{
		Enabled: true,
		IsNew:   m == nil,
	}

	if m != nil {
		data.Name = m.Name
		if m.Enabled != nil {
			data.Enabled = *m.Enabled
		}
		data.Filename = m.Filename
		data.Schedule = m.When
		data.Task = m.Do

		if len(m.From) > 0 {
			for _, src := range m.From {
				if src.URL != "" && data.URL == "" {
					data.URL = src.URL
				}
				if src.Search != "" && data.Search == "" {
					data.Search = src.Search
				}
			}
		}

		if len(m.Tell) > 0 {
			if ntfy, ok := m.Tell[0]["ntfy"]; ok {
				data.WebhookURL = fmt.Sprintf("%v", ntfy)
			} else if discord, ok := m.Tell[0]["discord"]; ok {
				data.WebhookURL = fmt.Sprintf("%v", discord)
			}
		}
	}

	f := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Minion Name").
				Value(&data.Name).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("name is required")
					}
					return nil
				}),
			huh.NewConfirm().
				Title("Enabled?").
				Value(&data.Enabled),
		).Title("1. Basics"),

		huh.NewGroup(
			huh.NewInput().
				Title("Schedule (e.g. 'daily @ 09:00', 'every 4h')").
				Value(&data.Schedule),
		).Title("2. Trigger"),

		huh.NewGroup(
			huh.NewInput().
				Title("Data Source URL (Optional)").
				Value(&data.URL),
			huh.NewInput().
				Title("Web Search Query (Optional)").
				Value(&data.Search),
		).Title("3. Input Sources"),

		huh.NewGroup(
			huh.NewText().
				Title("Task Instructions for AI (What to find?)").
				Value(&data.Task).
				Lines(5),
		).Title("4. AI Processing"),

		huh.NewGroup(
			huh.NewInput().
				Title("Webhook URL for Delivery (e.g. Ntfy, Discord)").
				Value(&data.WebhookURL),
		).Title("5. Delivery"),
	)

	return f, data
}

func saveForm(data *formData) error {
	filename := data.Filename
	if data.IsNew || filename == "" {
		safeName := strings.ToLower(strings.ReplaceAll(data.Name, " ", "_"))
		filename = safeName + ".yaml"
	}

	var sources []config.Source
	if strings.TrimSpace(data.URL) != "" {
		sources = append(sources, config.Source{URL: strings.TrimSpace(data.URL)})
	}
	if strings.TrimSpace(data.Search) != "" {
		sources = append(sources, config.Source{Search: strings.TrimSpace(data.Search), Limit: 3})
	}

	var tellTargets []map[string]interface{}
	if strings.TrimSpace(data.WebhookURL) != "" {
		target := make(map[string]interface{})
		if strings.Contains(data.WebhookURL, "ntfy.sh") {
			target["ntfy"] = strings.TrimSpace(data.WebhookURL)
		} else if strings.Contains(data.WebhookURL, "discord.com") {
			target["discord"] = strings.TrimSpace(data.WebhookURL)
		} else {
			target["http_request"] = strings.TrimSpace(data.WebhookURL)
		}
		tellTargets = append(tellTargets, target)
	}

	mConfig := config.MinionConfig{
		Name:     data.Name,
		Enabled:  &data.Enabled,
		When:     strings.TrimSpace(data.Schedule),
		From:     sources,
		Do:       strings.TrimSpace(data.Task),
		Tell:     tellTargets,
		Settings: config.Settings{Timeout: 15, Delay: 2},
	}

	yamlData, err := yaml.Marshal(mConfig)
	if err != nil {
		return err
	}

	path := filepath.Join(config.MinionsDir, filename)
	return os.WriteFile(path, yamlData, 0644)
}

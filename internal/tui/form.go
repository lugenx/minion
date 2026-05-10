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
	BrowseURL  string
	StudyTask  string
	DeliverURL string
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

		// Extract fields from Mission array
		for _, step := range m.Mission {
			if val, ok := step["schedule"]; ok {
				data.Schedule = fmt.Sprintf("%v", val)
			}
			if val, ok := step["search"]; ok {
				if searches, ok := val.([]interface{}); ok {
					var s []string
					for _, search := range searches {
						s = append(s, fmt.Sprintf("%v", search))
					}
					data.Search = strings.Join(s, ", ")
				}
			}
			if val, ok := step["browse"]; ok {
				if browses, ok := val.([]interface{}); ok {
					if len(browses) > 0 {
						if bMap, ok := browses[0].(map[string]interface{}); ok {
							if u, ok := bMap["url"]; ok {
								data.BrowseURL = fmt.Sprintf("%v", u)
							}
						}
					}
				}
			}
			if val, ok := step["study"]; ok {
				if sMap, ok := val.(map[string]interface{}); ok {
					if t, ok := sMap["task"]; ok {
						data.StudyTask = fmt.Sprintf("%v", t)
					}
				}
			}
			if val, ok := step["deliver"]; ok {
				if dArr, ok := val.([]interface{}); ok {
					if len(dArr) > 0 {
						if dMap, ok := dArr[0].(map[string]interface{}); ok {
							// Just grab the first key-value that is a string, typically ntfy, discord, or http_request
							for _, v := range dMap {
								if s, ok := v.(string); ok {
									data.DeliverURL = s
									break
								}
							}
						}
					}
				}
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
				Title("Web Search Queries (comma-separated)").
				Value(&data.Search),
			huh.NewInput().
				Title("Specific URL to Browse").
				Value(&data.BrowseURL),
		).Title("3. Data Sources"),

		huh.NewGroup(
			huh.NewText().
				Title("Study Instructions (Plain English task for the AI)").
				Value(&data.StudyTask).
				Lines(5),
		).Title("4. AI Brain"),

		huh.NewGroup(
			huh.NewInput().
				Title("Webhook URL for Delivery (e.g. Ntfy, Discord)").
				Value(&data.DeliverURL),
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

	// Build the mission array
	var mission []map[string]interface{}

	if strings.TrimSpace(data.Schedule) != "" {
		mission = append(mission, map[string]interface{}{"schedule": strings.TrimSpace(data.Schedule)})
	}

	if strings.TrimSpace(data.Search) != "" {
		searches := strings.Split(data.Search, ",")
		var sArr []string
		for _, s := range searches {
			clean := strings.TrimSpace(s)
			if clean != "" {
				sArr = append(sArr, clean)
			}
		}
		if len(sArr) > 0 {
			mission = append(mission, map[string]interface{}{
				"search": sArr,
				"limit":  3,
			})
		}
	}

	if strings.TrimSpace(data.BrowseURL) != "" {
		mission = append(mission, map[string]interface{}{
			"browse": []map[string]interface{}{
				{"url": strings.TrimSpace(data.BrowseURL)},
			},
		})
	}

	mission = append(mission, map[string]interface{}{
		"scrape": map[string]interface{}{
			"timeout": 15,
		},
	})

	if strings.TrimSpace(data.StudyTask) != "" {
		mission = append(mission, map[string]interface{}{
			"study": map[string]interface{}{
				"task": strings.TrimSpace(data.StudyTask),
			},
		})
	}

	if strings.TrimSpace(data.DeliverURL) != "" {
		target := "http_request"
		if strings.Contains(data.DeliverURL, "ntfy.sh") {
			target = "ntfy"
		} else if strings.Contains(data.DeliverURL, "discord.com") {
			target = "discord"
		}

		mission = append(mission, map[string]interface{}{
			"deliver": []map[string]interface{}{
				{target: strings.TrimSpace(data.DeliverURL)},
			},
		})
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

package tui

import (
	"os"
	"path/filepath"
	"strings"

	"minion/internal/config"
)

func getSuggestions(isAddStep bool, field, contextType, input string) (matches []string, isStrict bool) {
	var options []string
	isStrict = false

	if isAddStep {
		options = []string{"when", "from", "keep", "ignore", "do", "tell", "settings", "report"}
		isStrict = true
	} else if field == "When" {
		options = []string{
			"every 5m",
			"every 15m",
			"every 1h",
			"every 12h",
			"daily @ 09:00",
			"daily @ 17:00",
			"weekdays @ 09:00",
			"weekends @ 10:00",
			"monday,friday @ 12:00",
			"0 9 * * * (Cron Format)",
		}
		isStrict = false
	} else if field == "TellType" || field == "ReportType" {
		options = []string{"ntfy", "discord", "minion", "http_request", "file"}
		isStrict = true
	} else if field == "TellURL" || field == "ReportURL" {
		if contextType == "ntfy" {
			options = []string{"https://ntfy.sh/"}
		} else if contextType == "discord" {
			options = []string{"https://discord.com/api/webhooks/"}
		} else if contextType == "http_request" {
			options = []string{"https://"}
		} else if contextType == "minion" {
			options = []string{"worker.yaml"}
		} else if contextType == "file" {
			dataDir := filepath.Join(config.GlobalConfigDir, "data")
			entries, err := os.ReadDir(dataDir)
			if err == nil {
				for _, e := range entries {
					if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
						options = append(options, filepath.Join(dataDir, e.Name()))
					}
				}
			}
			if len(options) == 0 {
				options = append(options, filepath.Join(config.GlobalConfigDir, "data", "filename.yaml"))
			}
		}
		isStrict = false
	} else if field == "FromFile" {
		dataDir := filepath.Join(config.GlobalConfigDir, "data")
		entries, err := os.ReadDir(dataDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && (strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml")) {
					options = append(options, filepath.Join(dataDir, e.Name()))
				}
			}
		}
		if len(options) == 0 {
			options = append(options, filepath.Join(config.GlobalConfigDir, "data", "filename.yaml"))
		}
		isStrict = false
	} else {
		return nil, false
	}

	inputRaw := strings.TrimSpace(input)
	inputLower := strings.ToLower(inputRaw)
	
	for _, opt := range options {
		compareOpt := opt
		compareInput := inputRaw
		if isAddStep {
			compareInput = inputLower
		}
		
		if field == "FromFile" || (contextType == "file" && (field == "TellURL" || field == "ReportURL")) {
			if strings.Contains(strings.ToLower(opt), strings.ToLower(inputRaw)) {
				matches = append(matches, opt)
			}
		} else if strings.HasPrefix(compareOpt, compareInput) {
			matches = append(matches, opt)
		}
	}
	return matches, isStrict
}

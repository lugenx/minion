package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// MinionConfig represents a v2 task. It contains an array of raw pipeline steps.
type MinionConfig struct {
	Name     string                   `yaml:"name"`
	Enabled  *bool                    `yaml:"enabled"`
	Mission  []map[string]interface{} `yaml:"mission"`
	
	Filename string                   `yaml:"-"`
}

var (
	GlobalConfigDir string
	EnvPath         string
	MinionsDir      string
	LogsDir         string
	DBPath          string
	LogPath         string
	PIDPath         string
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "." 
	}

	GlobalConfigDir = filepath.Join(homeDir, ".config", "minion")
	EnvPath = filepath.Join(GlobalConfigDir, ".env")
	MinionsDir = filepath.Join(GlobalConfigDir, "minions")
	LogsDir = filepath.Join(GlobalConfigDir, "logs")
	DBPath = filepath.Join(GlobalConfigDir, "minion.db")
	LogPath = filepath.Join(GlobalConfigDir, "minion.log")
	PIDPath = filepath.Join(GlobalConfigDir, "minion.pid")
}

func EnsureDirectories() error {
	if err := os.MkdirAll(MinionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create minions directory: %w", err)
	}
	if err := os.MkdirAll(LogsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	if _, err := os.Stat(EnvPath); os.IsNotExist(err) {
		scaffoldExampleFiles()
	} else if _, err := os.Stat(filepath.Join(MinionsDir, "example.yaml")); os.IsNotExist(err) {
		scaffoldExampleFiles()
	}

	return nil
}

func scaffoldExampleFiles() {
	envContent := `# =================================================================
# MINION V2 SECRETS & CONFIGURATION
# =================================================================
# This file is loaded automatically by the minion engine.
# You can inject these variables into your minion YAML files using ${VAR_NAME}

# REQUIRED: Your OpenRouter API key for LLM evaluations
OPENROUTER_API_KEY=your_api_key_here

# OPTIONAL: The model you want the engine to use (Defaults to gpt-4o-mini if missing)
DEFAULT_MODEL=openai/gpt-4o-mini

# OPTIONAL: Example of custom secrets you can inject into your YAML files.
# If you run a private server that requires basic auth, store the credentials here.
# MY_USERNAME=admin
# MY_PASSWORD=super_secret_password
`
	_ = os.WriteFile(EnvPath, []byte(envContent), 0644)

	exampleContent := `# =================================================================
# MINION V2 REFERENCE GUIDE
# =================================================================
# Minion v2 operates on a linear stream. It gathers all URLs from 
# your search and browse blocks, and then passes them one-by-one 
# through the rest of the pipeline.

name: "Tech Event Tracker"
enabled: false # Set to true to let the daemon run this

mission:
  # --- 1. SCHEDULING SYNTAX ---
  # Groups:   "daily @ 09:00", "weekdays @ 18:00", "weekends @ 12:00"
  # Specific: "mon, wed, fri @ 17:30"
  # Interval: "every 30m", "every 12h"
  # Raw Cron: "*/15 * * * *"
  - schedule: "daily @ 09:00"

  # --- 2. GENERATORS (Gathering Data) ---
  - search: 
      - "latest open source AI models"
      - "AI startup news"
    limit: 3

  - browse:
      # If no rule is given, it just returns this exact link
      - url: "https://example.com/news"
      
      # If a rule is given, it browses the page and returns the matching sub-links
      # Supports full Regex!
      - url: "https://example.com/events"
        match: "/events/"

  # --- 3. THE PIPELINE (Runs one-by-one on the gathered links) ---
  
  # Fast Filter (Optional): Drop links before scraping them if they contain junk words.
  - filter: 
      drop_if_contains: ["webinar", "online only"]

  # Scrape: Download the actual HTML text of the clean links
  - scrape:
      timeout: 15
      delay: 2 # Max seconds to pause before scraping to avoid bot detection

  # Study: Tell the minion exactly what you are looking for
  - study:
      task: |
        Looking for tech events or any nerdy events.
        Must occur on Tuesday/Wednesday after 17:00, or Saturday after 15:00.
        Must be in-person in New York.

  # --- 4. DELIVERY ---
  # Send the results anywhere you want. You can define multiple targets!
  - deliver:
      - ntfy: "https://ntfy.sh/mytopic"
        # basic_auth:
        #   username: "${MY_USERNAME}"
        #   password: "${MY_PASSWORD}"
        
      # - discord: "https://discord.com/api/webhooks/..."
      
      # - minion: "my_worker_minion_filename"
      
      # - http_request: "https://custom-api.com/v1/data"
      #   method: "POST"
      #   headers:
      #     Content-Type: "application/json"
      #   payload_template: |
      #     {"custom_title": "{{.Title}}", "desc": "{{.Summary}}"}

# =================================================================
# EXAMPLE WORKER MINION
# =================================================================
# If you created a separate file called my_worker.yaml to receive 
# data from this minion, it would look like this:
#
# name: "Worker Bot"
# enabled: true
# mission:
#   - receive: "Tech Event Tracker"
#   - scrape: true
#   - study: true
#     task: "Summarize this."
#   - deliver:
#       - ntfy: "https://ntfy.sh/alerts"
`
	examplePath := filepath.Join(MinionsDir, "example.yaml")
	_ = os.WriteFile(examplePath, []byte(exampleContent), 0644)
}
func LoadEnv() error {
	EnsureDirectories()
	_ = godotenv.Load(EnvPath)
	return nil
}

func LoadMinion(filename string) (*MinionConfig, error) {
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename += ".yaml"
	}
	
	path := filename
	if !filepath.IsAbs(path) && !strings.Contains(path, string(os.PathSeparator)) {
		path = filepath.Join(MinionsDir, filename)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read minion file %s: %w", path, err)
	}

	expandedData := os.ExpandEnv(string(data))

	var m MinionConfig
	if err := yaml.Unmarshal([]byte(expandedData), &m); err != nil {
		return nil, fmt.Errorf("failed to parse minion YAML %s: %w", path, err)
	}
	
	if m.Enabled == nil {
		defaultEnabled := true
		m.Enabled = &defaultEnabled
	}
	
	m.Filename = filepath.Base(path)
	return &m, nil
}

func LoadAllMinions() ([]*MinionConfig, error) {
	var minions []*MinionConfig

	entries, err := os.ReadDir(MinionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return minions, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		m, err := LoadMinion(filepath.Join(MinionsDir, name))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %v\n", name, err)
			continue
		}
		minions = append(minions, m)
	}

	return minions, nil
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Source struct {
	URL         string `yaml:"url"`
	FollowLinks string `yaml:"follow_links"`
}

// UnmarshalYAML handles both flat strings and structured maps for Sources.
func (s *Source) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		s.URL = value.Value
		s.FollowLinks = ""
		return nil
	}

	type rawSource struct {
		URL         string `yaml:"url"`
		FollowLinks string `yaml:"follow_links"`
	}
	var rs rawSource
	if err := value.Decode(&rs); err != nil {
		return err
	}
	s.URL = rs.URL
	s.FollowLinks = rs.FollowLinks
	return nil
}

type BasicAuth struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type Webhook struct {
	URL       string            `yaml:"url"`
	Method    string            `yaml:"method"`
	BasicAuth *BasicAuth        `yaml:"basic_auth"`
	Headers   map[string]string `yaml:"headers"`
}

// MinionConfig represents a parsed YAML minion task.
type MinionConfig struct {
	Name           string   `yaml:"name"`
	Enabled        *bool    `yaml:"enabled"` // Pointer to distinguish missing vs false. Defaults to true.
	Schedule       string   `yaml:"schedule"`
	Webhook        *Webhook `yaml:"webhook"`
	Sources        []Source `yaml:"sources"`
	WebSearch      *Search  `yaml:"web_search"`
	SkipIfContains []string `yaml:"skip_if_contains"`
	Task           string   `yaml:"task"`

	// Metadata
	Filename string `yaml:"-"`
}

type Search struct {
	Queries           []string `yaml:"queries"`
	MaxResultsPerQuery int      `yaml:"max_results_per_query"`
}

var (
	GlobalConfigDir string
	EnvPath         string
	MinionsDir      string
	DBPath          string
	LogPath         string
	PIDPath         string
)

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "." // Fallback if home directory cannot be found
	}

	// Explicitly use ~/.config/minion for consistency across Linux and macOS
	GlobalConfigDir = filepath.Join(homeDir, ".config", "minion")
	EnvPath = filepath.Join(GlobalConfigDir, ".env")
	MinionsDir = filepath.Join(GlobalConfigDir, "minions")
	DBPath = filepath.Join(GlobalConfigDir, "minion.db")
	LogPath = filepath.Join(GlobalConfigDir, "minion.log")
	PIDPath = filepath.Join(GlobalConfigDir, "minion.pid")
}

// EnsureDirectories checks and creates the global config directories if they don't exist.
// It also scaffolds the example files for new users.
func EnsureDirectories() error {
	if err := os.MkdirAll(MinionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create minions directory: %w", err)
	}

	// Always scaffold if files are missing
	if _, err := os.Stat(EnvPath); os.IsNotExist(err) {
		scaffoldExampleFiles()
	} else if _, err := os.Stat(filepath.Join(MinionsDir, "example.yaml")); os.IsNotExist(err) {
		scaffoldExampleFiles()
	}

	return nil
}

func scaffoldExampleFiles() {
	// Create .env
	envContent := `# =================================================================
# MINION SECRETS & CONFIGURATION
# =================================================================
# This file is loaded automatically by the minion engine.
# You can inject these variables into your minion YAML files using ${VAR_NAME}

# REQUIRED: Your OpenRouter API key for LLM evaluations
OPENROUTER_API_KEY=your_api_key_here

# OPTIONAL: The model you want the engine to use (Defaults to gpt-4o-mini if missing)
DEFAULT_MODEL=openai/gpt-4o-mini

# OPTIONAL: Example of custom secrets you can inject into your YAML files.
# If you run a private server that requires basic auth, store the credentials here.
# MY_USERNAME=minion
# MY_PASSWORD=super_secret_password
`
	_ = os.WriteFile(EnvPath, []byte(envContent), 0644)

	// Create example.yaml
	exampleContent := `# =================================================================
# MINION REFERENCE GUIDE
# =================================================================
# This file shows every possible configuration option. 
# You can copy/paste blocks from here to create your own minions!

name: "Example"
enabled: false # Set to true to let the daemon run this

# --- SCHEDULING SYNTAX ---
# Groups:   "daily @ 09:00", "weekdays @ 18:00", "weekends @ 12:00"
# Specific: "mon, wed, fri @ 17:30"
# Interval: "every 30m", "every 12h"
# Raw Cron: "*/15 * * * *"
schedule: "daily @ 09:00"

# --- WEBHOOK NOTIFICATION ---
# Where to send the alert. Supports any generic HTTP endpoint.
webhook:
  url: "https://ntfy.sh/mytopic"
  # method: "POST" (default)
  
  # Standard HTTP Basic Authentication
  # You can securely inject secrets from your ~/.config/minion/.env file!
  # basic_auth:
  #   username: "${MY_USERNAME}"
  #   password: "${MY_PASSWORD}"
  
  # Optional custom headers
  # headers:
  #   X-Custom-Header: "value"

# --- SOURCES ---
sources:
  # Basic URL
  - "https://example.com/news"
  
  # Auto-link follower (Visits homepage, then scrapes every link containing the pattern)
  # - url: "https://example.com/products"
  #   follow_links: "/releases/"

# --- WEB SEARCH (Optional) ---
# Automatically searches DuckDuckGo and scrapes the top results
web_search:
  queries:
    - "latest open source AI models"
  max_results_per_query: 3

# --- FAST FILTER (Optional) ---
# If any of these words are found, drop the page immediately before sending to AI
skip_if_contains:
  - "paywall"
  - "subscribe to read"

# --- THE TASK ---
# Tell the Minion exactly what to look for and how to format it.
task: |
  Looking for official software release announcements for version 2.0 or higher.
  Must be released within the next 7 days.
  Write the summaries like a sarcastic tech journalist.
`
	examplePath := filepath.Join(MinionsDir, "example.yaml")
	_ = os.WriteFile(examplePath, []byte(exampleContent), 0644)
}

// LoadEnv loads the .env file from the global config directory.
func LoadEnv() error {
	EnsureDirectories()
	// Ignore error if file doesn't exist, we will check actual env vars later.
	_ = godotenv.Load(EnvPath)
	return nil
}

// LoadMinion reads and parses a specific minion YAML file.
func LoadMinion(filename string) (*MinionConfig, error) {
	// Add .yaml extension if missing
	if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
		filename += ".yaml"
	}
	
	// If it's just a name without a path, assume it's in the MinionsDir
	path := filename
	if !filepath.IsAbs(path) && !strings.Contains(path, string(os.PathSeparator)) {
		path = filepath.Join(MinionsDir, filename)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read minion file %s: %w", path, err)
	}

	// Expand environment variables (e.g. ${NTFY_AUTH})
	expandedData := os.ExpandEnv(string(data))

	var m MinionConfig
	if err := yaml.Unmarshal([]byte(expandedData), &m); err != nil {
		return nil, fmt.Errorf("failed to parse minion YAML %s: %w", path, err)
	}
	
	// Default Enabled to true if it was not specified in the YAML
	if m.Enabled == nil {
		defaultEnabled := true
		m.Enabled = &defaultEnabled
	}
	
	m.Filename = filepath.Base(path)

	return &m, nil
}

// LoadAllMinions reads all YAML files in the MinionsDir.
func LoadAllMinions() ([]*MinionConfig, error) {
	var minions []*MinionConfig

	entries, err := os.ReadDir(MinionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return minions, nil
		}
		return nil, fmt.Errorf("failed to read minions directory: %w", err)
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
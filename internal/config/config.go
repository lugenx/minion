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
	URL    string `yaml:"url,omitempty"`
	Render bool   `yaml:"render,omitempty"`
	Match  string `yaml:"match,omitempty"`
	Search string `yaml:"search,omitempty"`
	Limit  int    `yaml:"limit,omitempty"`
	Minion string `yaml:"minion,omitempty"`
	SourceType string `yaml:"-"`
}

type Settings struct {
	Timeout int `yaml:"timeout,omitempty"`
	Delay   int `yaml:"delay,omitempty"`
}

// MinionConfig represents a task schema.
type MinionConfig struct {
	Name     string                 `yaml:"name"`
	Enabled  *bool                  `yaml:"enabled"` // Legacy support
	When     string                 `yaml:"when,omitempty"`
	From     []Source               `yaml:"from"`
	Keep     []string               `yaml:"keep,omitempty"`
	Ignore   []string               `yaml:"ignore,omitempty"`
	Do       string                 `yaml:"do"`
	Tell     map[string]interface{} `yaml:"tell,omitempty"`
	Report   map[string]interface{} `yaml:"report,omitempty"`
	Settings Settings               `yaml:"settings,omitempty"`

	Filename string `yaml:"-"`
}

var (
	GlobalConfigDir string
	EnvPath         string
	MinionsDir      string
	LogsDir         string
	DBPath          string
	LogPath         string
	PIDPath         string
	MasksPath       string
)

var GlobalMasks []MaskConfig

type MaskConfig struct {
	Name        string `yaml:"name"`
	Pattern     string `yaml:"pattern"`
	Replacement string `yaml:"replacement"`
}

type MasksFile struct {
	Masks []MaskConfig `yaml:"masks"`
}

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
	MasksPath = filepath.Join(GlobalConfigDir, "masks.yaml")
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

	if _, err := os.Stat(MasksPath); os.IsNotExist(err) {
		scaffoldMasksFile()
	}

	return nil
}

func scaffoldExampleFiles() {
	envContent := `# =================================================================
# MINION SECRETS & CONFIGURATION
# =================================================================
# This file is loaded automatically by the minion engine.
# You can inject these variables into your minion YAML files using ${VAR_NAME}

# REQUIRED: Your OpenRouter API key for LLM evaluations
OPENROUTER_API_KEY=your_api_key_here

# OPTIONAL: The model you want the engine to use (Defaults to gpt-4o-mini if missing)
DEFAULT_MODEL=openai/gpt-4o-mini
`
	_ = os.WriteFile(EnvPath, []byte(envContent), 0644)

	exampleContent := `name: Tech Meetup Finder
when: daily @ 09:00

from:
  # 1. Direct Scrape
  - url: https://events.example.com

  # 2. Rendered Scrape (Headless Browser)
  - url: https://community.example.com/discover
    render: true

  # 3. Crawler (Finds matching links, then scrapes them)
  - url: https://calendar.example.com/events
    match: /events

  # 4. Search Generator
  - search: programming meetups this week
    limit: 3

keep:
  - golang
  - rust
  - artificial intelligence
  - open source

ignore:
  - webinar
  - online only
  - virtual

do: Summarize each event with date, location, and topic.

tell:
  ntfy: https://ntfy.sh/your-topic-here
  markdown: true

report:
  # ntfy: https://ntfy.sh/your-topic-here

settings:
  timeout: 15
  delay: 2
`
	examplePath := filepath.Join(MinionsDir, "example.yaml")
	_ = os.WriteFile(examplePath, []byte(exampleContent), 0644)
}

func scaffoldMasksFile() {
	masksContent := `# =================================================================
# MINION SNAPSHOT MASKS (PREVENT DUPLICATES)
# =================================================================
# Minion prevents duplicate messages by taking a "snapshot" of the page.
# If the website has constantly changing numbers (like "views" or "mins ago"),
# the snapshot breaks and you get duplicates.
#
# Use this file to MASK the changes you don't care about. 
# Minion will place a static mask over the dynamic text before taking the snapshot!

masks:
  - name: "Relative Time (The Ticking Clocks)"
    pattern: '(?i)\b\d+\s*(secs?|seconds?|mins?|minutes?|hrs?|hours?|days?|weeks?|months?|years?)\s*ago\b'
    replacement: '<TIME_AGO>'

  - name: "Engagement Metrics"
    pattern: '(?i)\b\d+\s*(views?|comments?|likes?|replies|retweets|shares)\b'
    replacement: '<METRIC>'

  - name: "Parenthetical Counters"
    pattern: '\(\s*\d+\s*\)'
    replacement: '<COUNT>'

  - name: "Dynamic Updates"
    pattern: '(?i)(?:last\s*)?updated\s*(?:today|yesterday|now|\d+)'
    replacement: '<UPDATED>'

  - name: "And X Others"
    pattern: '(?i)(?:and\s+)?\d+\s+others?'
    replacement: '<AND_OTHERS>'

  - name: "Attendees / Going (Event sites)"
    pattern: '(?i)\b\d+\s*(?:people\s*)?(?:going|interested|attending|registered)\b'
    replacement: '<ATTENDEES>'

  - name: "Active/Online Users (Forums)"
    pattern: '(?i)\b\d+\s*(?:users?\s*)?(?:online|active|viewing)(?:\s*now)?\b'
    replacement: '<ACTIVE_USERS>'

  - name: "Review Counters"
    pattern: '(?i)\b\d+(?:,\d+)?\s*(?:customer\s*)?reviews?\b'
    replacement: '<REVIEW_COUNT>'

  - name: "Follower/Subscriber Counters"
    pattern: '(?i)\b\d+(?:[kKmM])?\s*(?:followers?|subscribers?)\b'
    replacement: '<FOLLOWER_COUNT>'
`
	_ = os.WriteFile(MasksPath, []byte(masksContent), 0644)
}

func LoadMasks() error {
	data, err := os.ReadFile(MasksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // It's ok if it doesn't exist
		}
		return fmt.Errorf("failed to read masks file: %w", err)
	}

	var mf MasksFile
	if err := yaml.Unmarshal(data, &mf); err != nil {
		return fmt.Errorf("failed to parse masks file: %w", err)
	}

	GlobalMasks = mf.Masks
	return nil
}

func LoadEnv() error {
	EnsureDirectories()
	LoadMasks()
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

	var m MinionConfig
	if err := yaml.Unmarshal(data, &m); err != nil {
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

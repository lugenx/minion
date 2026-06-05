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
	URL        string `yaml:"url,omitempty"`
	Render     bool   `yaml:"render,omitempty"`
	Match      string `yaml:"follow,omitempty"`
	Search     string `yaml:"search,omitempty"`
	Limit      int    `yaml:"limit,omitempty"`
	Minion     string `yaml:"minion,omitempty"`
	Command    string `yaml:"command,omitempty"`
	File       string `yaml:"file,omitempty"`
	SourceType string `yaml:"-"`
}

func (s *Source) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		URL     string `yaml:"url"`
		Render  bool   `yaml:"render"`
		Match   string `yaml:"match"`
		Follow  string `yaml:"follow"`
		Search  string `yaml:"search"`
		Limit   int    `yaml:"limit"`
		Minion  string `yaml:"minion"`
		Command string `yaml:"command"`
		File    string `yaml:"file"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	s.URL = raw.URL
	s.Render = raw.Render
	s.Search = raw.Search
	s.Limit = raw.Limit
	s.Minion = raw.Minion
	s.Command = raw.Command
	s.File = raw.File
	if raw.Follow != "" {
		s.Match = raw.Follow
	} else {
		s.Match = raw.Match
	}
	switch {
	case raw.File != "":
		s.SourceType = "file"
	case raw.Command != "":
		s.SourceType = "command"
	case raw.Minion != "":
		s.SourceType = "minion"
	case raw.Search != "" || raw.Limit != 0:
		s.SourceType = "search"
	default:
		s.SourceType = "url"
	}
	return nil
}

type Settings struct {
	Timeout int    `yaml:"timeout,omitempty"`
	Delay   int    `yaml:"delay,omitempty"`
	Model   string `yaml:"model,omitempty"`
}

// MinionConfig represents a task schema.
type MinionConfig struct {
	Name     string                   `yaml:"name"`
	Enabled  *bool                    `yaml:"enabled"` // Legacy support
	When     string                   `yaml:"when,omitempty"`
	From     []Source                 `yaml:"from"`
	Keep     []string                 `yaml:"keep,omitempty"`
	Ignore   []string                 `yaml:"ignore,omitempty"`
	Do       string                   `yaml:"do"`
	Tell     []map[string]interface{} `yaml:"tell,omitempty"`
	Report   []map[string]interface{} `yaml:"report,omitempty"`
	Settings Settings                 `yaml:"settings,omitempty"`

	Filename string `yaml:"-"`
}

// UnmarshalYAML supports both the old flat map format (tell: {ntfy: ...})
// and the new array format (tell: [{ntfy: ...}, {discord: ...}]).
func (m *MinionConfig) UnmarshalYAML(value *yaml.Node) error {
	var raw struct {
		Name     string      `yaml:"name"`
		Enabled  *bool       `yaml:"enabled"`
		When     string      `yaml:"when"`
		From     []Source    `yaml:"from"`
		Keep     []string    `yaml:"keep"`
		Ignore   []string    `yaml:"ignore"`
		Do       string      `yaml:"do"`
		Tell     interface{} `yaml:"tell"`
		Report   interface{} `yaml:"report"`
		Settings Settings    `yaml:"settings"`
	}
	if err := value.Decode(&raw); err != nil {
		return err
	}
	m.Name = raw.Name
	m.Enabled = raw.Enabled
	m.When = raw.When
	m.From = raw.From
	m.Keep = raw.Keep
	m.Ignore = raw.Ignore
	m.Do = raw.Do
	m.Settings = raw.Settings

	if raw.Tell != nil {
		switch v := raw.Tell.(type) {
		case map[string]interface{}:
			if len(v) > 0 {
				m.Tell = []map[string]interface{}{v}
			}
		case []interface{}:
			for _, item := range v {
				if m2, ok := item.(map[string]interface{}); ok {
					m.Tell = append(m.Tell, m2)
				}
			}
		}
	}
	if raw.Report != nil {
		switch v := raw.Report.(type) {
		case map[string]interface{}:
			if len(v) > 0 {
				m.Report = []map[string]interface{}{v}
			}
		case []interface{}:
			for _, item := range v {
				if m2, ok := item.(map[string]interface{}); ok {
					m.Report = append(m.Report, m2)
				}
			}
		}
	}
	return nil
}

var (
	GlobalConfigDir string
	EnvPath         string
	MinionsDir      string
	LogsDir         string
	DataDir         string
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
	DataDir = filepath.Join(GlobalConfigDir, "data")
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
	if err := os.MkdirAll(DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
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

# OPTIONAL: Default model used by all minions (can be overridden per-minion in settings.model)
DEFAULT_MODEL=google/gemma-4-31b-it
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
    follow: /events

  # 4. Search Generator
  - search: programming meetups this week
    limit: 3

  # 5. Read records from a file (YAML stream or plain text)
  # - file: ~/.config/minion/data/input.yaml

  # 6. Run a system command
  - command: df -h

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
  - ntfy: https://ntfy.sh/your-topic-here
    markdown: true
    basic_auth:
      username: "${NTFY_USER}"
      password: "${NTFY_PASS}"

  # - file: ~/.config/minion/data/output.yaml
  #   capacity: 100

report:
  - ntfy: https://ntfy.sh/your-topic-here
  # - file: ~/.config/minion/data/report.yaml

settings:
  timeout: 15
  delay: 2
  model: google/gemma-4-31b-it
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

func ensureEnvVar(key, val string) {
	data, err := os.ReadFile(EnvPath)
	if err != nil {
		return
	}
	if strings.Contains(string(data), key+"=") {
		return
	}
	f, err := os.OpenFile(EnvPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString("\n# Set to false to disable the character art in TUI\n" + key + "=" + val + "\n")
}

func LoadEnv() error {
	EnsureDirectories()
	LoadMasks()
	_ = godotenv.Load(EnvPath)
	ensureEnvVar("MINION_CHARACTER", "true")
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

package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseInline builds an ephemeral minion configuration from ordered key=value
// assignments. Source options apply to the most recently declared source.
func ParseInline(assignments []string) (*MinionConfig, error) {
	enabled := true
	m := &MinionConfig{
		Name:     "Inline Run",
		Enabled:  &enabled,
		Filename: "inline",
	}

	seen := make(map[string]bool)
	currentSource := -1
	currentSourceOptions := make(map[string]bool)

	for i, assignment := range assignments {
		key, value, ok := strings.Cut(assignment, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("argument %d must be a key=value assignment", i+1)
		}

		switch key {
		case "name":
			if err := markInlineScalar(seen, key); err != nil {
				return nil, err
			}
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("name cannot be empty")
			}
			m.Name = value

		case "do":
			if err := markInlineScalar(seen, key); err != nil {
				return nil, err
			}
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("do cannot be empty")
			}
			m.Do = value

		case "keep":
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("keep cannot be empty")
			}
			m.Keep = append(m.Keep, value)

		case "ignore":
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("ignore cannot be empty")
			}
			m.Ignore = append(m.Ignore, value)

		case "from.search", "from.url", "from.command", "from.file":
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("%s cannot be empty", key)
			}

			source := Source{}
			switch key {
			case "from.search":
				source.Search = value
				source.SourceType = "search"
			case "from.url":
				source.URL = value
				source.SourceType = "url"
			case "from.command":
				source.Command = value
				source.SourceType = "command"
			case "from.file":
				source.File = value
				source.SourceType = "file"
			}
			m.From = append(m.From, source)
			currentSource = len(m.From) - 1
			currentSourceOptions = make(map[string]bool)

		case "from.limit":
			if err := requireInlineSource(m, currentSource, "search", key); err != nil {
				return nil, err
			}
			limit, err := strconv.Atoi(value)
			if err != nil || limit <= 0 {
				return nil, fmt.Errorf("%s must be a positive integer", key)
			}
			if currentSourceOptions[key] {
				return nil, fmt.Errorf("%s is assigned more than once for the current source", key)
			}
			m.From[currentSource].Limit = limit
			currentSourceOptions[key] = true

		case "from.render":
			if err := requireInlineSource(m, currentSource, "url", key); err != nil {
				return nil, err
			}
			if currentSourceOptions[key] {
				return nil, fmt.Errorf("%s is assigned more than once for the current source", key)
			}
			render, err := parseInlineBool(value)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			m.From[currentSource].Render = render
			currentSourceOptions[key] = true

		case "from.follow":
			if err := requireInlineSource(m, currentSource, "url", key); err != nil {
				return nil, err
			}
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("%s cannot be empty", key)
			}
			if currentSourceOptions[key] {
				return nil, fmt.Errorf("%s is assigned more than once for the current source", key)
			}
			m.From[currentSource].Match = value
			currentSourceOptions[key] = true

		case "tell.file":
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("%s cannot be empty", key)
			}
			m.Tell = append(m.Tell, map[string]interface{}{"file": value})

		case "settings.timeout":
			if err := markInlineScalar(seen, key); err != nil {
				return nil, err
			}
			timeout, err := strconv.Atoi(value)
			if err != nil || timeout <= 0 {
				return nil, fmt.Errorf("%s must be a positive integer", key)
			}
			m.Settings.Timeout = timeout

		case "settings.model":
			if err := markInlineScalar(seen, key); err != nil {
				return nil, err
			}
			if strings.TrimSpace(value) == "" {
				return nil, fmt.Errorf("%s cannot be empty", key)
			}
			m.Settings.Model = value

		default:
			return nil, fmt.Errorf("unknown inline key %q", key)
		}
	}

	if len(m.From) == 0 && m.Do == "" {
		return nil, fmt.Errorf("inline run requires at least one source or do assignment")
	}

	return m, nil
}

func markInlineScalar(seen map[string]bool, key string) error {
	if seen[key] {
		return fmt.Errorf("%s is assigned more than once", key)
	}
	seen[key] = true
	return nil
}

func requireInlineSource(m *MinionConfig, index int, sourceType, key string) error {
	if index < 0 {
		return fmt.Errorf("%s requires a preceding from.%s source", key, sourceType)
	}
	if m.From[index].SourceType != sourceType {
		return fmt.Errorf("%s applies only to the current from.%s source", key, sourceType)
	}
	return nil
}

func parseInlineBool(value string) (bool, error) {
	switch value {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("must be true or false")
	}
}

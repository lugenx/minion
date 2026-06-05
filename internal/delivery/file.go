package delivery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"minion/internal/types"
)

func WriteFileLine(path string, item *types.Item, capacity *int) error {
	if path == "" {
		return nil
	}
	if capacity != nil && *capacity < 0 {
		return fmt.Errorf("capacity must be >= 0, got %d", *capacity)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create parent dirs: %w", err)
	}

	ts := item.Timestamp
	if ts == "" {
		ts = time.Now().Format(time.RFC3339)
	}

	rec := types.FileRecord{
		Title:     item.Title,
		URL:       item.URL,
		Summary:   item.Summary,
		Text:   item.Text,
		Timestamp: ts,
	}

	data, err := yaml.Marshal(&rec)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	fi, statErr := os.Stat(path)
	fileExists := statErr == nil && fi.Size() > 0

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer f.Close()

	if fileExists {
		if _, err := f.WriteString("---\n"); err != nil {
			return fmt.Errorf("failed to write separator: %w", err)
		}
	}

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	if capacity != nil {
		if err := trimFile(path, *capacity); err != nil {
			return fmt.Errorf("failed to trim: %w", err)
		}
	}

	return nil
}

func trimFile(path string, capacity int) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	parts := splitDocs(string(raw))

	var docs []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			docs = append(docs, p)
		}
	}

	if len(docs) <= capacity {
		return nil
	}

	docs = docs[len(docs)-capacity:]

	var buf strings.Builder
	for i, doc := range docs {
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.WriteString(strings.TrimRight(doc, "\n\r"))
		buf.WriteByte('\n')
	}

	return os.WriteFile(path, []byte(buf.String()), 0644)
}

func splitDocs(raw string) []string {
	var docs []string
	var buf []string
	for _, line := range strings.Split(raw, "\n") {
		if line == "---" || strings.HasPrefix(line, "--- ") {
			if len(buf) > 0 {
				docs = append(docs, strings.Join(buf, "\n"))
				buf = nil
			}
			continue
		}
		buf = append(buf, line)
	}
	if len(buf) > 0 {
		docs = append(docs, strings.Join(buf, "\n"))
	}
	return docs
}

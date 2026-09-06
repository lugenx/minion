package delivery

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"minion/internal/types"
)

// MarshalFileRecordYAML converts an Item to the canonical FileRecord YAML representation.
func MarshalFileRecordYAML(item *types.Item) ([]byte, error) {
	ts := item.Timestamp
	if ts == "" {
		ts = time.Now().Format(time.RFC3339)
	}

	rec := types.FileRecord{
		Title:     item.Title,
		URL:       item.URL,
		Summary:   item.Summary,
		Text:      item.Text,
		Timestamp: ts,
	}

	data, err := yaml.Marshal(&rec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal record: %w", err)
	}
	return data, nil
}

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

	data, err := MarshalFileRecordYAML(item)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for append: %w", err)
	}
	defer f.Close()
	if err := lockFile(f); err != nil {
		return fmt.Errorf("failed to lock file: %w", err)
	}
	defer unlockFile(f)

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	payload := data
	if fi.Size() > 0 {
		payload = append([]byte("---\n"), data...)
	}
	if _, err := f.Write(payload); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	if capacity != nil {
		if err := trimFile(f, *capacity); err != nil {
			return fmt.Errorf("failed to trim: %w", err)
		}
	}

	return nil
}

func trimFile(file *os.File, capacity int) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	raw, err := io.ReadAll(file)
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

	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err = file.WriteString(buf.String())
	return err
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

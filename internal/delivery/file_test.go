package delivery

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"minion/internal/types"
)

func ptr(i int) *int { return &i }

func tempFilePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "output.yaml")
}

func TestWriteFileLineUsesSharedYAMLSerialization(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{
		Title:     "shared",
		URL:       "https://example.com",
		Text:      "same bytes",
		Timestamp: "2026-09-05T12:00:00Z",
	}
	want := []byte("title: shared\nurl: https://example.com\ntext: same bytes\ntimestamp: \"2026-09-05T12:00:00Z\"\n")
	serialized, err := MarshalFileRecordYAML(item)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(serialized, want) {
		t.Fatalf("shared serialization changed the FileRecord YAML contract:\ngot:\n%s\nwant:\n%s", serialized, want)
	}
	if err := WriteFileLine(path, item, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, serialized) {
		t.Fatalf("file output differs from shared serialization:\ngot:\n%s\nwant:\n%s", got, serialized)
	}
}

func TestWriteFileLine_ItemWithText(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{
		Text: "raw scraped content",
	}
	if err := WriteFileLine(path, item, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "text: raw scraped content") {
		t.Fatalf("expected text: field in YAML, got: %q", s)
	}
}

func TestWriteFileLine_ItemWithoutText(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{
		Title:   "analyzed title",
		Summary: "a summary",
	}
	if err := WriteFileLine(path, item, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "text:") {
		t.Fatalf("expected no text: field in YAML (post-do), got: %q", s)
	}
	if !strings.Contains(s, "title: analyzed title") {
		t.Fatalf("expected title: field, got: %q", s)
	}
	if !strings.Contains(s, "summary: a summary") {
		t.Fatalf("expected summary: field, got: %q", s)
	}
}

func TestWriteFileLine_ItemWithAllFields(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{
		Title:     "t",
		URL:       "https://example.com",
		Summary:   "s",
		Text:      "x",
		Timestamp: "now",
	}
	if err := WriteFileLine(path, item, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "title: t") {
		t.Fatalf("missing title: %q", s)
	}
	if !strings.Contains(s, "url: https://example.com") {
		t.Fatalf("missing url: %q", s)
	}
	if !strings.Contains(s, "summary: s") {
		t.Fatalf("missing summary: %q", s)
	}
	if !strings.Contains(s, "text: x") {
		t.Fatalf("missing text: %q", s)
	}
	if !strings.Contains(s, "timestamp: now") {
		t.Fatalf("missing timestamp: %q", s)
	}
}

func TestWriteFileLine_EmptyItem(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{}
	if err := WriteFileLine(path, item, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	// Should have only timestamp (auto-set) and no other fields
	if !strings.Contains(s, "timestamp:") {
		t.Fatalf("expected timestamp to be set automatically, got: %q", s)
	}
	if strings.Contains(s, "title:") || strings.Contains(s, "url:") || strings.Contains(s, "summary:") || strings.Contains(s, "text:") {
		t.Fatalf("expected only timestamp for empty item, got: %q", s)
	}
}

func TestWriteFileLine_MultipleDocs(t *testing.T) {
	path := tempFilePath(t)
	item1 := &types.Item{Text: "doc one"}
	item2 := &types.Item{Text: "doc two"}
	item3 := &types.Item{Text: "doc three"}

	for _, item := range []*types.Item{item1, item2, item3} {
		if err := WriteFileLine(path, item, nil); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	// Should have 3 documents, separated by ---
	parts := strings.Split(s, "---\n")
	if len(parts) != 3 {
		t.Fatalf("expected 3 docs separated by ---, got %d parts: %q", len(parts), s)
	}
}

func TestWriteFileLine_TrimToCapacity(t *testing.T) {
	path := tempFilePath(t)

	for range 5 {
		item := &types.Item{Text: "doc"}
		if err := WriteFileLine(path, item, ptr(3)); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	parts := strings.Split(s, "---\n")
	if len(parts) > 3 {
		t.Fatalf("expected at most 3 docs after trim, got %d", len(parts))
	}
}

func TestWriteFileLine_ZeroCapacity(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{Text: "doc"}

	for range 3 {
		if err := WriteFileLine(path, item, ptr(0)); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty file after writes with capacity=0, got: %q", string(data))
	}
}

func TestWriteFileLine_NegativeCapacity(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{Text: "doc"}

	err := WriteFileLine(path, item, ptr(-1))
	if err == nil {
		t.Fatal("expected error for negative capacity, got nil")
	}
}

func TestWriteFileLine_ItemWithURL(t *testing.T) {
	path := tempFilePath(t)
	item := &types.Item{
		Title: "t",
		URL:   "https://example.com",
	}
	if err := WriteFileLine(path, item, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "title: t") {
		t.Fatalf("expected title, got: %q", s)
	}
	if !strings.Contains(s, "url: https://example.com") {
		t.Fatalf("expected url, got: %q", s)
	}
	if strings.Contains(s, "text:") {
		t.Fatalf("expected no text: for item without Text, got: %q", s)
	}
}

func TestSplitDocs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{"empty", "", 1}, // splitDocs returns [""] for empty input; caller filters empties
		{"single doc", "hello world", 1},
		{"two docs", "doc1\n---\ndoc2", 2},
		{"three docs", "a\n---\nb\n---\nc", 3},
		{"indented separator is content", "a\n ---\nb", 1},
		{"separator with trailing space", "a\n--- \nb", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitDocs(tt.raw)
			if len(got) != tt.want {
				t.Fatalf("splitDocs(%q) got %d docs, want %d", tt.raw, len(got), tt.want)
			}
		})
	}
}

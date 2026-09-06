package delivery

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

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

func TestWriteFileLineProcessHelper(t *testing.T) {
	if os.Getenv("MINION_FILE_WRITER_HELPER") != "1" {
		return
	}

	target := os.Getenv("MINION_FILE_WRITER_TARGET")
	value := os.Getenv("MINION_FILE_WRITER_VALUE")
	readyDir := os.Getenv("MINION_FILE_WRITER_READY_DIR")
	barrier := os.Getenv("MINION_FILE_WRITER_BARRIER")
	if err := os.WriteFile(filepath.Join(readyDir, value), nil, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	for {
		if _, err := os.Stat(barrier); err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	var capacity *int
	if raw := os.Getenv("MINION_FILE_WRITER_CAPACITY"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		capacity = &parsed
	}
	item := &types.Item{Text: value, Timestamp: "2026-09-06T00:00:00Z"}
	if err := WriteFileLine(target, item, capacity); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func runConcurrentFileWriters(t *testing.T, preseed bool, capacity *int) []types.FileRecord {
	t.Helper()
	const writers = 24
	dir := t.TempDir()
	target := filepath.Join(dir, "shared.yaml")
	readyDir := filepath.Join(dir, "ready")
	barrier := filepath.Join(dir, "go")
	if err := os.Mkdir(readyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if preseed {
		if err := WriteFileLine(target, &types.Item{Text: "seed", Timestamp: "2026-09-06T00:00:00Z"}, capacity); err != nil {
			t.Fatal(err)
		}
	}

	type child struct {
		value string
		cmd   *exec.Cmd
		out   bytes.Buffer
	}
	children := make([]*child, 0, writers)
	for i := range writers {
		value := fmt.Sprintf("writer-%02d", i)
		c := &child{value: value}
		c.cmd = exec.Command(os.Args[0], "-test.run=^TestWriteFileLineProcessHelper$")
		c.cmd.Env = append(os.Environ(),
			"MINION_FILE_WRITER_HELPER=1",
			"MINION_FILE_WRITER_TARGET="+target,
			"MINION_FILE_WRITER_VALUE="+value,
			"MINION_FILE_WRITER_READY_DIR="+readyDir,
			"MINION_FILE_WRITER_BARRIER="+barrier,
		)
		if capacity != nil {
			c.cmd.Env = append(c.cmd.Env, "MINION_FILE_WRITER_CAPACITY="+strconv.Itoa(*capacity))
		}
		c.cmd.Stdout = &c.out
		c.cmd.Stderr = &c.out
		if err := c.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		children = append(children, c)
	}

	deadline := time.Now().Add(10 * time.Second)
	for {
		entries, err := os.ReadDir(readyDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) == writers {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d/%d child writers became ready", len(entries), writers)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := os.WriteFile(barrier, nil, 0644); err != nil {
		t.Fatal(err)
	}

	for _, c := range children {
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("%s failed: %v\n%s", c.value, err, c.out.String())
		}
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var records []types.FileRecord
	for {
		var record types.FileRecord
		err := decoder.Decode(&record)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("corrupt YAML stream: %v\n%s", err, data)
		}
		records = append(records, record)
	}
	return records
}

func TestWriteFileLine_ConcurrentProcessesCreateValidStream(t *testing.T) {
	for _, preseed := range []bool{false, true} {
		t.Run(fmt.Sprintf("preseed=%t", preseed), func(t *testing.T) {
			records := runConcurrentFileWriters(t, preseed, nil)
			want := 24
			if preseed {
				want++
			}
			if len(records) != want {
				t.Fatalf("got %d records, want %d", len(records), want)
			}
			seen := make(map[string]bool)
			for _, record := range records {
				seen[record.Text] = true
			}
			for i := range 24 {
				value := fmt.Sprintf("writer-%02d", i)
				if !seen[value] {
					t.Fatalf("missing structured record %q", value)
				}
			}
		})
	}
}

func TestWriteFileLine_ConcurrentCapacityTrimStaysValid(t *testing.T) {
	capacity := 10
	records := runConcurrentFileWriters(t, true, &capacity)
	if len(records) != capacity {
		t.Fatalf("got %d records after concurrent capacity trim, want %d", len(records), capacity)
	}
	seen := make(map[string]bool)
	for _, record := range records {
		if seen[record.Text] {
			t.Fatalf("duplicate record after concurrent trim: %q", record.Text)
		}
		seen[record.Text] = true
	}
}

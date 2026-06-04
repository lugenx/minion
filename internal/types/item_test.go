package types

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFileRecordYAML_OmitAll(t *testing.T) {
	rec := FileRecord{}
	data, err := yaml.Marshal(&rec)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 && string(data) != "{}\n" && string(data) != "null\n" {
		t.Fatalf("expected empty YAML for zero-value FileRecord, got: %q", string(data))
	}
}

func TestFileRecordYAML_TextOnly(t *testing.T) {
	rec := FileRecord{Text: "hello world"}
	data, err := yaml.Marshal(&rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s != "text: hello world\n" {
		t.Fatalf("expected only text field, got: %q", s)
	}
}

func TestFileRecordYAML_TitleOnly(t *testing.T) {
	rec := FileRecord{Title: "my title"}
	data, err := yaml.Marshal(&rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s != "title: my title\n" {
		t.Fatalf("expected only title field, got: %q", s)
	}
}

func TestFileRecordYAML_AllFields(t *testing.T) {
	rec := FileRecord{
		Title:     "t",
		URL:       "u",
		Summary:   "s",
		Text:      "x",
		Timestamp: "now",
	}
	data, err := yaml.Marshal(&rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s != "title: t\nurl: u\nsummary: s\ntext: x\ntimestamp: now\n" {
		t.Fatalf("unexpected YAML: %q", s)
	}
}

func TestFileRecordYAML_MixedFields(t *testing.T) {
	rec := FileRecord{Title: "t", Summary: "s"}
	data, err := yaml.Marshal(&rec)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s != "title: t\nsummary: s\n" {
		t.Fatalf("expected only title+summary, got: %q", s)
	}
}

func TestFileRecordYAML_RoundTrip(t *testing.T) {
	rec := FileRecord{
		Title:     "title",
		URL:       "https://example.com",
		Summary:   "a summary",
		Text:      "some text",
		Timestamp: "2024-01-01T00:00:00Z",
	}
	data, err := yaml.Marshal(&rec)
	if err != nil {
		t.Fatal(err)
	}
	var decoded FileRecord
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Title != rec.Title {
		t.Fatalf("Title: got %q, want %q", decoded.Title, rec.Title)
	}
	if decoded.URL != rec.URL {
		t.Fatalf("URL: got %q, want %q", decoded.URL, rec.URL)
	}
	if decoded.Summary != rec.Summary {
		t.Fatalf("Summary: got %q, want %q", decoded.Summary, rec.Summary)
	}
	if decoded.Text != rec.Text {
		t.Fatalf("Text: got %q, want %q", decoded.Text, rec.Text)
	}
	if decoded.Timestamp != rec.Timestamp {
		t.Fatalf("Timestamp: got %q, want %q", decoded.Timestamp, rec.Timestamp)
	}
}

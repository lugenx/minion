package minion

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"minion/internal/config"
	"minion/internal/types"
)

func TestValidateRunArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "daemon"},
		{name: "saved minion", args: []string{"research"}},
		{name: "single inline", args: []string{"from.url=https://example.com"}},
		{name: "multiple inline", args: []string{"from.search=test", "from.limit=2"}},
		{name: "too many filenames", args: []string{"one", "two"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRunArgs(nil, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateRunArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteInlineResultsYAML(t *testing.T) {
	items := []types.Item{
		{URL: "https://example.com/one", Text: "first page\n\nsecond paragraph", Timestamp: "2026-09-05T12:00:00Z"},
		{Command: "printf second", Text: "second result", Timestamp: "2026-09-05T12:01:00Z"},
		{FilePath: "/tmp/input.yaml", Title: "Saved title", Summary: "Saved summary", Timestamp: "2026-09-05T12:00:00Z"},
		{Title: "Analyzed", URL: "https://example.com/final", Summary: "Final-stage summary", Timestamp: "2026-09-05T12:02:00Z"},
	}
	var out bytes.Buffer
	if err := writeInlineResults(&out, items); err != nil {
		t.Fatal(err)
	}

	decoder := yaml.NewDecoder(&out)
	var got []types.FileRecord
	for {
		var rec types.FileRecord
		err := decoder.Decode(&rec)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode YAML stream: %v", err)
		}
		got = append(got, rec)
	}

	want := []types.FileRecord{
		{URL: "https://example.com/one", Text: "first page\n\nsecond paragraph", Timestamp: "2026-09-05T12:00:00Z"},
		{Text: "second result", Timestamp: "2026-09-05T12:01:00Z"},
		{Title: "Saved title", Summary: "Saved summary", Timestamp: "2026-09-05T12:00:00Z"},
		{Title: "Analyzed", URL: "https://example.com/final", Summary: "Final-stage summary", Timestamp: "2026-09-05T12:02:00Z"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected records: %#v, want %#v", got, want)
	}
}

func TestRunInlineCommandRawOutput(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	m, err := config.ParseInline([]string{"from.command=printf inline-ok"})
	if err != nil {
		t.Fatal(err)
	}
	err = runInlineConfig(cmd, m)
	if err != nil {
		t.Fatalf("runInline() error = %v, stderr=%q", err, stderr.String())
	}
	var rec types.FileRecord
	if err := yaml.Unmarshal(stdout.Bytes(), &rec); err != nil {
		t.Fatalf("stdout is not valid FileRecord YAML: %v\n%s", err, stdout.String())
	}
	if rec.Text != "inline-ok" || rec.Timestamp == "" {
		t.Fatalf("unexpected stdout record: %#v", rec)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunInlineCommandFailure(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	m, err := config.ParseInline([]string{"from.command=printf failed-output >&2; exit 7"})
	if err != nil {
		t.Fatal(err)
	}

	err = runInlineConfig(cmd, m)
	if err == nil || !strings.Contains(err.Error(), "1 error") {
		t.Fatalf("expected failed inline run, got %v", err)
	}
	var rec types.FileRecord
	if err := yaml.Unmarshal(stdout.Bytes(), &rec); err != nil {
		t.Fatalf("failure stdout is not valid FileRecord YAML: %v\n%s", err, stdout.String())
	}
	if !strings.Contains(rec.Text, "failed-output") {
		t.Fatalf("expected command output, got %#v", rec)
	}
	if !strings.Contains(stderr.String(), "status 7") {
		t.Fatalf("expected command failure diagnostic, got %q", stderr.String())
	}
}

func TestRunInlineTellFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.yaml")
	m, err := config.ParseInline([]string{
		"from.command=printf saved-result",
		"tell.file=" + path,
	})
	if err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := runInlineConfig(cmd, m); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "text: saved-result") {
		t.Fatalf("unexpected tell.file output: %s", data)
	}
}

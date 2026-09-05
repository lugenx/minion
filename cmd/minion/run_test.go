package minion

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

func TestWriteInlineResultsRaw(t *testing.T) {
	items := []types.Item{
		{URL: "https://example.com/one", Text: "first page"},
		{Command: "printf second", Text: "second result"},
		{FilePath: "/tmp/input.yaml", Title: "Saved title", Summary: "Saved summary", Timestamp: "2026-09-05T12:00:00Z"},
	}
	var out bytes.Buffer
	if err := writeInlineResults(&out, false, items); err != nil {
		t.Fatal(err)
	}
	want := "Source: https://example.com/one\nfirst page\n\nSource: command: printf second\nsecond result\n\nSource: file: /tmp/input.yaml\nSaved title\nSaved summary\nTimestamp: 2026-09-05T12:00:00Z\n"
	if out.String() != want {
		t.Fatalf("unexpected output:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestWriteInlineResultsAnalyzed(t *testing.T) {
	items := []types.Item{
		{Title: "First", URL: "https://example.com/one", Summary: "A summary."},
		{Title: "Second", Summary: "Another summary."},
	}
	var out bytes.Buffer
	if err := writeInlineResults(&out, true, items); err != nil {
		t.Fatal(err)
	}
	want := "First\nURL: https://example.com/one\nA summary.\n\nSecond\nAnother summary.\n"
	if out.String() != want {
		t.Fatalf("unexpected output:\n%s\nwant:\n%s", out.String(), want)
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
	if !strings.Contains(stdout.String(), "Source: command: printf inline-ok\ninline-ok") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
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
	if !strings.Contains(stdout.String(), "failed-output") {
		t.Fatalf("expected command output, got %q", stdout.String())
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

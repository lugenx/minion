package config

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseInline(t *testing.T) {
	m, err := ParseInline([]string{
		"name=Research",
		"from.search=SearXNG Hermes Agent",
		"from.limit=5",
		"from.url=https://example.com/?a=b",
		"from.render=true",
		"from.follow=/docs",
		"from.command=printf 'a=b'",
		"from.file=/tmp/input.yaml",
		"keep=agents",
		"keep=search",
		"ignore=paywall",
		"do=Find setup instructions=a comparison",
		"tell.file=/tmp/output.yaml",
		"settings.timeout=20",
		"settings.model=provider/model",
	})
	if err != nil {
		t.Fatal(err)
	}

	if m.Name != "Research" || m.Filename != "inline" {
		t.Fatalf("unexpected inline identity: name=%q filename=%q", m.Name, m.Filename)
	}
	if m.Enabled == nil || !*m.Enabled {
		t.Fatal("inline minion should be enabled")
	}
	if len(m.From) != 4 {
		t.Fatalf("expected 4 sources, got %d", len(m.From))
	}
	if m.From[0].Search != "SearXNG Hermes Agent" || m.From[0].Limit != 5 {
		t.Fatalf("unexpected search source: %#v", m.From[0])
	}
	if m.From[1].URL != "https://example.com/?a=b" || !m.From[1].Render || m.From[1].Match != "/docs" {
		t.Fatalf("unexpected URL source: %#v", m.From[1])
	}
	if m.From[2].Command != "printf 'a=b'" || m.From[3].File != "/tmp/input.yaml" {
		t.Fatalf("unexpected command/file sources: %#v", m.From)
	}
	if !reflect.DeepEqual(m.Keep, []string{"agents", "search"}) || !reflect.DeepEqual(m.Ignore, []string{"paywall"}) {
		t.Fatalf("unexpected filters: keep=%v ignore=%v", m.Keep, m.Ignore)
	}
	if m.Do != "Find setup instructions=a comparison" {
		t.Fatalf("value after first equals was not preserved: %q", m.Do)
	}
	if len(m.Tell) != 1 || m.Tell[0]["file"] != "/tmp/output.yaml" {
		t.Fatalf("unexpected tell targets: %#v", m.Tell)
	}
	if m.Settings.Timeout != 20 || m.Settings.Model != "provider/model" {
		t.Fatalf("unexpected settings: %#v", m.Settings)
	}
}

func TestParseInlineMultipleSources(t *testing.T) {
	m, err := ParseInline([]string{
		"from.search=first",
		"from.limit=2",
		"from.search=second",
		"from.limit=4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.From) != 2 || m.From[0].Limit != 2 || m.From[1].Limit != 4 {
		t.Fatalf("source-local limits attached incorrectly: %#v", m.From)
	}
}

func TestParseInlineDoOnly(t *testing.T) {
	m, err := ParseInline([]string{"do=Answer a question"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Do != "Answer a question" || len(m.From) != 0 {
		t.Fatalf("unexpected do-only config: %#v", m)
	}
}

func TestParseInlineErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing equals", args: []string{"from.url"}, want: "key=value"},
		{name: "empty key", args: []string{"=value"}, want: "key=value"},
		{name: "unknown key", args: []string{"from.minion=worker"}, want: "unknown inline key"},
		{name: "no work", args: []string{"name=Test"}, want: "requires at least one source"},
		{name: "empty source", args: []string{"from.url="}, want: "cannot be empty"},
		{name: "whitespace source", args: []string{"from.url=   "}, want: "cannot be empty"},
		{name: "empty do", args: []string{"do="}, want: "cannot be empty"},
		{name: "empty model", args: []string{"do=test", "settings.model="}, want: "cannot be empty"},
		{name: "local before source", args: []string{"from.limit=2"}, want: "preceding from.search"},
		{name: "local wrong source", args: []string{"from.url=https://example.com", "from.limit=2"}, want: "only to the current from.search"},
		{name: "invalid limit", args: []string{"from.search=test", "from.limit=zero"}, want: "positive integer"},
		{name: "invalid render", args: []string{"from.url=https://example.com", "from.render=yes"}, want: "true or false"},
		{name: "duplicate render false", args: []string{"from.url=https://example.com", "from.render=false", "from.render=true"}, want: "more than once"},
		{name: "duplicate follow", args: []string{"from.url=https://example.com", "from.follow=/a", "from.follow=/b"}, want: "more than once"},
		{name: "duplicate scalar", args: []string{"do=one", "do=two"}, want: "more than once"},
		{name: "invalid timeout", args: []string{"do=test", "settings.timeout=0"}, want: "positive integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseInline(tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
		})
	}
}
